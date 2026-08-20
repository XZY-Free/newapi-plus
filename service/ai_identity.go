package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 节点内 Identity Snapshot 缓存
//
// 统一按 token_id 读取完整 Identity Snapshot（第二批运行时中间件依赖此方法），
// 并在任何治理实体被修改后失效当前节点缓存。缓存仅存在于单节点内存，
// 不跨进程；跨节点一致性由后续批次的运行时刷新/分布式缓存承担。
// ---------------------------------------------------------------------------

type identitySnapshotCacheStore struct {
	sync.RWMutex
	m map[int]*types.IdentitySnapshot
}

var identitySnapshotCache = &identitySnapshotCacheStore{m: map[int]*types.IdentitySnapshot{}}

// GetIdentitySnapshotByTokenID 按 token_id 统一读取完整身份快照，带节点内缓存。
// 若该 token 未登记 Profile，返回 (nil, nil)。
func GetIdentitySnapshotByTokenID(tokenID int) (*types.IdentitySnapshot, error) {
	if tokenID <= 0 {
		return nil, nil
	}
	identitySnapshotCache.RLock()
	if s, ok := identitySnapshotCache.m[tokenID]; ok {
		identitySnapshotCache.RUnlock()
		// 返回深拷贝，调用者修改不得污染缓存。
		return s.Clone(), nil
	}
	identitySnapshotCache.RUnlock()

	s, err := buildIdentitySnapshot(tokenID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	identitySnapshotCache.Lock()
	identitySnapshotCache.m[tokenID] = s
	identitySnapshotCache.Unlock()
	return s.Clone(), nil
}

// invalidateIdentitySnapshotCache 清除当前节点全部身份快照缓存。
func invalidateIdentitySnapshotCache() {
	identitySnapshotCache.Lock()
	identitySnapshotCache.m = map[int]*types.IdentitySnapshot{}
	identitySnapshotCache.Unlock()
}

func buildIdentitySnapshot(tokenID int) (*types.IdentitySnapshot, error) {
	var profile model.AIIdentityProfile
	if err := model.DB.Where("token_id = ?", tokenID).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	s := &types.IdentitySnapshot{
		ProfileID:         profile.Id,
		TokenID:           tokenID,
		Enabled:           profile.Enabled,
		IdentityMode:      profile.IdentityMode,
		AttributionTarget: profile.AttributionTargetType,
		IdentityAssurance: profile.IdentityAssurance,
		Environment:       profile.Environment,
		CallerID:          profile.CallerId,
		CallerName:        profile.CallerName,
		RateLimit: types.ProfileRateLimit{
			Enabled:       profile.RateLimitEnabled,
			WindowSeconds: profile.RateLimitWindowSeconds,
			MaxRequests:   profile.RateLimitMaxRequests,
		},
	}

	// 引用表查询遇到缺失必须返回错误，不得静默 continue 形成貌似可信的半快照。
	if profile.PrincipalId > 0 {
		var principal model.AIPrincipal
		if err := model.DB.First(&principal, profile.PrincipalId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("快照引用的使用主体(id=%d)不存在", profile.PrincipalId)
			}
			return nil, err
		}
		s.PrincipalID = principal.Id
		s.PrincipalCode = principal.PrincipalCode
		s.PrincipalName = principal.PrincipalName
		s.PrincipalEnabled = principal.Enabled
		if principal.BusinessDomainID > 0 {
			var domain model.AIBusinessDomain
			if err := model.DB.First(&domain, principal.BusinessDomainID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("快照引用的业务领域(id=%d)不存在", principal.BusinessDomainID)
				}
				return nil, err
			}
			s.UsageDomainID = domain.Id
			s.UsageDomainCode = domain.DomainCode
			s.UsageDomainName = domain.DomainName
		}
		if principal.UsageTeamID > 0 {
			var team model.AIUsageTeam
			if err := model.DB.First(&team, principal.UsageTeamID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("快照引用的使用团队(id=%d)不存在", principal.UsageTeamID)
				}
				return nil, err
			}
			s.UsageTeamID = team.Id
			s.UsageTeamCode = team.TeamCode
			s.UsageTeamName = team.TeamName
		}
	}
	if profile.CredentialPurposeId > 0 {
		var purpose model.AICredentialPurpose
		if err := model.DB.First(&purpose, profile.CredentialPurposeId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("快照引用的凭证用途(id=%d)不存在", profile.CredentialPurposeId)
			}
			return nil, err
		}
		s.CredentialPurposeID = purpose.Id
		s.CredentialPurposeCode = purpose.PurposeCode
		s.CredentialPurposeName = purpose.PurposeName
		s.CredentialPurposeEnabled = purpose.Enabled
	}

	// 已启用的 App Bindings → Application + Domain + Owner Team 快照。
	var bindings []model.AIIdentityAppBinding
	if err := model.DB.Where("profile_id = ? AND enabled = ?", profile.Id, true).Find(&bindings).Error; err != nil {
		return nil, err
	}
	for _, b := range bindings {
		var app model.AIApplication
		if err := model.DB.First(&app, b.AppId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("快照引用的应用(id=%d)不存在", b.AppId)
			}
			return nil, err
		}
		snap := types.SnapshotApplication{
			ApplicationID:  app.Id,
			AppCode:        app.AppCode,
			AppName:        app.AppName,
			AppEnabled:     app.Enabled,
			BindingEnabled: b.Enabled,
		}
		if app.BusinessDomainID > 0 {
			var domain model.AIBusinessDomain
			if err := model.DB.First(&domain, app.BusinessDomainID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("快照引用的业务领域(id=%d)不存在", app.BusinessDomainID)
				}
				return nil, err
			}
			snap.BusinessDomainID = domain.Id
			snap.BusinessDomainCode = domain.DomainCode
			snap.BusinessDomainName = domain.DomainName
		}
		if app.OwnerTeamID > 0 {
			var team model.AIOwnerTeam
			if err := model.DB.First(&team, app.OwnerTeamID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("快照引用的负责团队(id=%d)不存在", app.OwnerTeamID)
				}
				return nil, err
			}
			snap.OwnerTeamID = team.Id
			snap.OwnerTeamCode = team.TeamCode
			snap.OwnerTeamName = team.TeamName
		}
		s.Applications = append(s.Applications, snap)
	}

	// 当前可用于验证的签名密钥元数据（6.17）：status in ACTIVE/RETIRING、revoked_at=0、
	// not_before<=now、expires_at=0 或 >now。未来生效、已到期、REVOKED 或非法状态
	// 一律不进入快照。绝不含密文/明文。
	now := common.GetTimestamp()
	var keys []model.AIIdentitySigningKey
	if err := model.DB.
		Select("id", "profile_id", "key_id", "status", "not_before", "expires_at", "revoked_at").
		Where("profile_id = ? AND status IN ? AND revoked_at = 0 AND not_before <= ? AND (expires_at = 0 OR expires_at > ?)",
			profile.Id, []string{constant.SigningKeyStatusActive, constant.SigningKeyStatusRetiring}, now, now).
		Order("id asc").Find(&keys).Error; err != nil {
		return nil, err
	}
	s.HasActiveSigningKey = false
	for _, k := range keys {
		if k.Status == constant.SigningKeyStatusActive {
			s.HasActiveSigningKey = true
		}
		s.SigningKeys = append(s.SigningKeys, types.SigningKeyMeta{
			KeyId:     k.KeyId,
			Status:    k.Status,
			NotBefore: k.NotBefore,
			ExpiresAt: k.ExpiresAt,
			RevokedAt: k.RevokedAt,
		})
	}

	// NewAPI Token 安全姿态摘要（完整 flags，绝不含 Key）。
	token, err := model.GetTokenById(tokenID)
	if err != nil {
		return nil, fmt.Errorf("快照引用的 NewAPI Token(id=%d)不存在", tokenID)
	}
	s.TokenSecurity = &types.CredentialSecurity{
		IPRestricted:     len(token.GetIpLimits()) > 0,
		ModelRestricted:  token.ModelLimitsEnabled && strings.TrimSpace(token.ModelLimits) != "",
		QuotaRestricted:  !token.UnlimitedQuota,
		ExpiryConfigured: token.ExpiredTime != -1,
		UnlimitedQuota:   token.UnlimitedQuota,
		CreatedTime:      token.CreatedTime,
		ExpiredTime:      token.ExpiredTime,
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// 主数据 code 校验
// ---------------------------------------------------------------------------

// validateDomainCode 校验 domain_code / app_code 字符集（文档 6.2 / 6.7）。
func validateDomainCode(code string) error {
	if len(code) < 2 || len(code) > 64 {
		return errors.New("编号 code 长度必须在 2 到 64 字符之间")
	}
	if !isLowerAlphaChar(code[0]) {
		return errors.New("编号 code 首字符必须为小写英文字母")
	}
	for i := 1; i < len(code); i++ {
		c := code[i]
		if !isLowerAlphaChar(c) && !isDigitChar(c) && c != '.' && c != '_' && c != '-' {
			return errors.New("编号 code 仅允许小写字母、数字、.、_、-，不允许空格")
		}
	}
	return nil
}

// validateSimpleCode 校验 team_code / principal_code / purpose_code：非空、无空格、长度受限。
func validateSimpleCode(code string, maxLen int, label string) error {
	if code == "" {
		return fmt.Errorf("%s 不能为空", label)
	}
	if len(code) > maxLen {
		return fmt.Errorf("%s 长度不能超过 %d 字符", label, maxLen)
	}
	if strings.ContainsAny(code, " \t\r\n") {
		return fmt.Errorf("%s 不允许包含空格", label)
	}
	return nil
}

func isLowerAlphaChar(c byte) bool { return c >= 'a' && c <= 'z' }
func isDigitChar(c byte) bool      { return c >= '0' && c <= '9' }

// validateNameLen 校验名称字段长度（varchar 按 Unicode 字符数计，非字节；不依赖
// MySQL 静默截断）。code 等 ASCII 字段仍按字节规则校验。
func validateNameLen(name string, label string) error {
	if utf8.RuneCountInString(name) > 128 {
		return fmt.Errorf("%s 长度不能超过 128 字符", label)
	}
	return nil
}

// validateTrimmedNonEmpty 校验 update 场景下显式提供的名称不得为空白
// （禁止 Trim 后写空串）。
func validateTrimmedNonEmpty(name, label string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s 不能为空", label)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 业务领域（Business Domain）
// ---------------------------------------------------------------------------

func CreateBusinessDomain(code, name string) (*model.AIBusinessDomain, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if err := validateDomainCode(code); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("业务领域名称不能为空")
	}
	if err := validateNameLen(name, "业务领域名称"); err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	domain := &model.AIBusinessDomain{
		DomainCode: code,
		DomainName: name,
		Enabled:    true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := model.DB.Create(domain).Error; err != nil {
		return nil, wrapUniqueError(err, "业务领域编号已存在")
	}
	invalidateIdentitySnapshotCache()
	return domain, nil
}

func UpdateBusinessDomain(id int, name string, enabled *bool) (*model.AIBusinessDomain, error) {
	var domain model.AIBusinessDomain
	if err := model.DB.First(&domain, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("业务领域不存在")
		}
		return nil, err
	}
	// code 创建后不可修改。
	updates := map[string]interface{}{"updated_at": common.GetTimestamp()}
	if name != "" {
		if err := validateTrimmedNonEmpty(name, "业务领域名称"); err != nil {
			return nil, err
		}
		domain.DomainName = strings.TrimSpace(name)
		if err := validateNameLen(domain.DomainName, "业务领域名称"); err != nil {
			return nil, err
		}
		updates["domain_name"] = domain.DomainName
	}
	if enabled != nil {
		domain.Enabled = *enabled
		updates["enabled"] = *enabled
	}
	if err := model.DB.Model(&domain).Updates(updates).Error; err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return &domain, nil
}

func GetBusinessDomain(id int) (*model.AIBusinessDomain, error) {
	var d model.AIBusinessDomain
	if err := model.DB.First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("业务领域不存在")
		}
		return nil, err
	}
	return &d, nil
}

func ListBusinessDomains(page, pageSize int, keyword string, enabled *bool) ([]model.AIBusinessDomain, int64, error) {
	q := model.DB.Model(&model.AIBusinessDomain{})
	if keyword != "" {
		q = q.Where("(domain_code LIKE ? OR domain_name LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	return listPaged[model.AIBusinessDomain](q, page, pageSize)
}

// ---------------------------------------------------------------------------
// Owner Team（AI 应用建设/维护/运营负责团队）
// ---------------------------------------------------------------------------

func CreateOwnerTeam(code, name string) (*model.AIOwnerTeam, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if err := validateSimpleCode(code, 64, "负责团队编号"); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("负责团队名称不能为空")
	}
	if err := validateNameLen(name, "负责团队名称"); err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	team := &model.AIOwnerTeam{TeamCode: code, TeamName: name, Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := model.DB.Create(team).Error; err != nil {
		return nil, wrapUniqueError(err, "负责团队编号已存在")
	}
	invalidateIdentitySnapshotCache()
	return team, nil
}

func UpdateOwnerTeam(id int, name string, enabled *bool) (*model.AIOwnerTeam, error) {
	var team model.AIOwnerTeam
	if err := model.DB.First(&team, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("负责团队不存在")
		}
		return nil, err
	}
	updates := map[string]interface{}{"updated_at": common.GetTimestamp()}
	if name != "" {
		if err := validateTrimmedNonEmpty(name, "负责团队名称"); err != nil {
			return nil, err
		}
		team.TeamName = strings.TrimSpace(name)
		if err := validateNameLen(team.TeamName, "负责团队名称"); err != nil {
			return nil, err
		}
		updates["team_name"] = team.TeamName
	}
	if enabled != nil {
		team.Enabled = *enabled
		updates["enabled"] = *enabled
	}
	if err := model.DB.Model(&team).Updates(updates).Error; err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return &team, nil
}

func GetOwnerTeam(id int) (*model.AIOwnerTeam, error) {
	var t model.AIOwnerTeam
	if err := model.DB.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("负责团队不存在")
		}
		return nil, err
	}
	return &t, nil
}

func ListOwnerTeams(page, pageSize int, keyword string, enabled *bool) ([]model.AIOwnerTeam, int64, error) {
	q := model.DB.Model(&model.AIOwnerTeam{})
	if keyword != "" {
		q = q.Where("(team_code LIKE ? OR team_name LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	return listPaged[model.AIOwnerTeam](q, page, pageSize)
}

// ---------------------------------------------------------------------------
// Usage Team（凭证使用人所属团队）
// ---------------------------------------------------------------------------

func CreateUsageTeam(code, name string) (*model.AIUsageTeam, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if err := validateSimpleCode(code, 64, "使用团队编号"); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("使用团队名称不能为空")
	}
	if err := validateNameLen(name, "使用团队名称"); err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	team := &model.AIUsageTeam{TeamCode: code, TeamName: name, Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := model.DB.Create(team).Error; err != nil {
		return nil, wrapUniqueError(err, "使用团队编号已存在")
	}
	invalidateIdentitySnapshotCache()
	return team, nil
}

func UpdateUsageTeam(id int, name string, enabled *bool) (*model.AIUsageTeam, error) {
	var team model.AIUsageTeam
	if err := model.DB.First(&team, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("使用团队不存在")
		}
		return nil, err
	}
	updates := map[string]interface{}{"updated_at": common.GetTimestamp()}
	if name != "" {
		if err := validateTrimmedNonEmpty(name, "使用团队名称"); err != nil {
			return nil, err
		}
		team.TeamName = strings.TrimSpace(name)
		if err := validateNameLen(team.TeamName, "使用团队名称"); err != nil {
			return nil, err
		}
		updates["team_name"] = team.TeamName
	}
	if enabled != nil {
		team.Enabled = *enabled
		updates["enabled"] = *enabled
	}
	if err := model.DB.Model(&team).Updates(updates).Error; err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return &team, nil
}

func ListUsageTeams(page, pageSize int, keyword string, enabled *bool) ([]model.AIUsageTeam, int64, error) {
	q := model.DB.Model(&model.AIUsageTeam{})
	if keyword != "" {
		q = q.Where("(team_code LIKE ? OR team_name LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	return listPaged[model.AIUsageTeam](q, page, pageSize)
}

// ---------------------------------------------------------------------------
// Principal（使用主体）
// ---------------------------------------------------------------------------

func CreatePrincipal(code, name string, businessDomainID, usageTeamID int) (*model.AIPrincipal, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if err := validateSimpleCode(code, 128, "使用主体编号"); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("使用主体名称不能为空")
	}
	if err := validateNameLen(name, "使用主体名称"); err != nil {
		return nil, err
	}
	// 创建时 Domain 与 Usage Team 必须 enabled。
	if err := requireEnabledDomain(businessDomainID); err != nil {
		return nil, err
	}
	if err := requireEnabledUsageTeam(usageTeamID); err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	p := &model.AIPrincipal{
		PrincipalCode:    code,
		PrincipalName:    name,
		PrincipalType:    constant.PrincipalTypePerson,
		BusinessDomainID: businessDomainID,
		UsageTeamID:      usageTeamID,
		Enabled:          true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := model.DB.Create(p).Error; err != nil {
		return nil, wrapUniqueError(err, "使用主体编号已存在")
	}
	invalidateIdentitySnapshotCache()
	return p, nil
}

func UpdatePrincipal(id int, name string, businessDomainID, usageTeamID int, enabled *bool) (*model.AIPrincipal, error) {
	var p model.AIPrincipal
	if err := model.DB.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("使用主体不存在")
		}
		return nil, err
	}
	updates := map[string]interface{}{"updated_at": common.GetTimestamp()}
	if name != "" {
		if err := validateTrimmedNonEmpty(name, "使用主体名称"); err != nil {
			return nil, err
		}
		p.PrincipalName = strings.TrimSpace(name)
		if err := validateNameLen(p.PrincipalName, "使用主体名称"); err != nil {
			return nil, err
		}
		updates["principal_name"] = p.PrincipalName
	}
	if businessDomainID > 0 {
		if err := requireEnabledDomain(businessDomainID); err != nil {
			return nil, err
		}
		p.BusinessDomainID = businessDomainID
		updates["business_domain_id"] = businessDomainID
	}
	if usageTeamID > 0 {
		if err := requireEnabledUsageTeam(usageTeamID); err != nil {
			return nil, err
		}
		p.UsageTeamID = usageTeamID
		updates["usage_team_id"] = usageTeamID
	}
	if enabled != nil {
		p.Enabled = *enabled
		updates["enabled"] = *enabled
	}
	if err := model.DB.Model(&p).Updates(updates).Error; err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return &p, nil
}

func ListPrincipals(page, pageSize int, keyword string, enabled *bool, businessDomainID, usageTeamID int) ([]model.AIPrincipal, int64, error) {
	q := model.DB.Model(&model.AIPrincipal{})
	if keyword != "" {
		q = q.Where("(principal_code LIKE ? OR principal_name LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	if businessDomainID > 0 {
		q = q.Where("business_domain_id = ?", businessDomainID)
	}
	if usageTeamID > 0 {
		q = q.Where("usage_team_id = ?", usageTeamID)
	}
	return listPaged[model.AIPrincipal](q, page, pageSize)
}

func GetPrincipal(id int) (*model.AIPrincipal, error) {
	var p model.AIPrincipal
	if err := model.DB.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("使用主体不存在")
		}
		return nil, err
	}
	return &p, nil
}

func GetCredentialPurpose(id int) (*model.AICredentialPurpose, error) {
	var p model.AICredentialPurpose
	if err := model.DB.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("凭证用途不存在")
		}
		return nil, err
	}
	return &p, nil
}

// ---------------------------------------------------------------------------
// Credential Purpose（凭证登记用途）
// ---------------------------------------------------------------------------

func CreateCredentialPurpose(code, name, purposeType string) (*model.AICredentialPurpose, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if err := validateSimpleCode(code, 64, "用途编号"); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("用途名称不能为空")
	}
	if err := validateNameLen(name, "用途名称"); err != nil {
		return nil, err
	}
	if !constant.PurposeTypeValid(purposeType) {
		return nil, errors.New("用途类型非法")
	}
	now := common.GetTimestamp()
	p := &model.AICredentialPurpose{
		PurposeCode: code,
		PurposeName: name,
		PurposeType: purposeType,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := model.DB.Create(p).Error; err != nil {
		return nil, wrapUniqueError(err, "用途编号已存在")
	}
	invalidateIdentitySnapshotCache()
	return p, nil
}

func UpdateCredentialPurpose(id int, name string, purposeType string, enabled *bool) (*model.AICredentialPurpose, error) {
	var p model.AICredentialPurpose
	if err := model.DB.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("凭证用途不存在")
		}
		return nil, err
	}
	updates := map[string]interface{}{"updated_at": common.GetTimestamp()}
	if name != "" {
		if err := validateTrimmedNonEmpty(name, "用途名称"); err != nil {
			return nil, err
		}
		p.PurposeName = strings.TrimSpace(name)
		if err := validateNameLen(p.PurposeName, "用途名称"); err != nil {
			return nil, err
		}
		updates["purpose_name"] = p.PurposeName
	}
	if purposeType != "" {
		if !constant.PurposeTypeValid(purposeType) {
			return nil, errors.New("用途类型非法")
		}
		p.PurposeType = purposeType
		updates["purpose_type"] = purposeType
	}
	if enabled != nil {
		p.Enabled = *enabled
		updates["enabled"] = *enabled
	}
	if err := model.DB.Model(&p).Updates(updates).Error; err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return &p, nil
}

func ListCredentialPurposes(page, pageSize int, keyword string, enabled *bool, purposeType string) ([]model.AICredentialPurpose, int64, error) {
	q := model.DB.Model(&model.AICredentialPurpose{})
	if keyword != "" {
		q = q.Where("(purpose_code LIKE ? OR purpose_name LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	if purposeType != "" {
		q = q.Where("purpose_type = ?", purposeType)
	}
	return listPaged[model.AICredentialPurpose](q, page, pageSize)
}

// ---------------------------------------------------------------------------
// AI Application（企业 AI 应用）
// ---------------------------------------------------------------------------

func CreateApplication(code, name string, businessDomainID, ownerTeamID int) (*model.AIApplication, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if err := validateDomainCode(code); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("应用名称不能为空")
	}
	if err := validateNameLen(name, "应用名称"); err != nil {
		return nil, err
	}
	if err := requireEnabledDomain(businessDomainID); err != nil {
		return nil, err
	}
	if err := requireEnabledOwnerTeam(ownerTeamID); err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	app := &model.AIApplication{
		AppCode:          code,
		AppName:          name,
		BusinessDomainID: businessDomainID,
		OwnerTeamID:      ownerTeamID,
		Enabled:          true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := model.DB.Create(app).Error; err != nil {
		return nil, wrapUniqueError(err, "应用编号已存在")
	}
	invalidateIdentitySnapshotCache()
	return app, nil
}

func UpdateApplication(id int, name string, businessDomainID, ownerTeamID int, enabled *bool) (*model.AIApplication, error) {
	var app model.AIApplication
	if err := model.DB.First(&app, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("应用不存在")
		}
		return nil, err
	}
	updates := map[string]interface{}{"updated_at": common.GetTimestamp()}
	if name != "" {
		if err := validateTrimmedNonEmpty(name, "应用名称"); err != nil {
			return nil, err
		}
		app.AppName = strings.TrimSpace(name)
		if err := validateNameLen(app.AppName, "应用名称"); err != nil {
			return nil, err
		}
		updates["app_name"] = app.AppName
	}
	if businessDomainID > 0 {
		if err := requireEnabledDomain(businessDomainID); err != nil {
			return nil, err
		}
		app.BusinessDomainID = businessDomainID
		updates["business_domain_id"] = businessDomainID
	}
	if ownerTeamID > 0 {
		if err := requireEnabledOwnerTeam(ownerTeamID); err != nil {
			return nil, err
		}
		app.OwnerTeamID = ownerTeamID
		updates["owner_team_id"] = ownerTeamID
	}
	if enabled != nil {
		app.Enabled = *enabled
		updates["enabled"] = *enabled
	}
	if err := model.DB.Model(&app).Updates(updates).Error; err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return &app, nil
}

func ListApplications(page, pageSize int, keyword string, enabled *bool, businessDomainID, ownerTeamID int) ([]model.AIApplication, int64, error) {
	q := model.DB.Model(&model.AIApplication{})
	if keyword != "" {
		q = q.Where("(app_code LIKE ? OR app_name LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	if businessDomainID > 0 {
		q = q.Where("business_domain_id = ?", businessDomainID)
	}
	if ownerTeamID > 0 {
		q = q.Where("owner_team_id = ?", ownerTeamID)
	}
	return listPaged[model.AIApplication](q, page, pageSize)
}

func GetApplication(id int) (*model.AIApplication, error) {
	var app model.AIApplication
	if err := model.DB.First(&app, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("应用不存在")
		}
		return nil, err
	}
	return &app, nil
}

// ---------------------------------------------------------------------------
// 被引用实体 enabled 门禁
// ---------------------------------------------------------------------------

func requireEnabledDomain(domainID int) error {
	if domainID <= 0 {
		return errors.New("业务领域不能为空")
	}
	var d model.AIBusinessDomain
	if err := model.DB.First(&d, domainID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("业务领域不存在")
		}
		return err
	}
	if !d.Enabled {
		return errors.New("已停用的业务领域不能分配给新的应用或使用主体")
	}
	return nil
}

func requireEnabledOwnerTeam(teamID int) error {
	if teamID <= 0 {
		return errors.New("负责团队不能为空")
	}
	var t model.AIOwnerTeam
	if err := model.DB.First(&t, teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("负责团队不存在")
		}
		return err
	}
	if !t.Enabled {
		return errors.New("已停用的负责团队不能分配给新的应用")
	}
	return nil
}

func requireEnabledUsageTeam(teamID int) error {
	if teamID <= 0 {
		return errors.New("使用团队不能为空")
	}
	var t model.AIUsageTeam
	if err := model.DB.First(&t, teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("使用团队不存在")
		}
		return err
	}
	if !t.Enabled {
		return errors.New("已停用的使用团队不能分配给新的使用主体")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Identity Profile 组合约束、启用门禁、绑定替换
// ---------------------------------------------------------------------------

// validateProfileCombination 依据文档 6.8 组合约束校验。bindingCount 为当前（或待创建）绑定数。
//
// ACTIVE 签名密钥的存在性属于“启用门禁”，不在此校验（创建/更新 disabled Profile 时
// 尚未生成密钥是合法的，见 6.10 规则 7：DYNAMIC/HYBRID 启用前才要求 ACTIVE Key）。
func validateProfileCombination(p *model.AIIdentityProfile, bindingCount int) error {
	switch {
	case p.IdentityMode == constant.IdentityModeStatic && p.AttributionTargetType == constant.AttributionTargetPrincipal:
		if p.IdentityAssurance != constant.IdentityAssuranceCredentialOnly {
			return errors.New("STATIC/PRINCIPAL 身份可信等级必须为 CREDENTIAL_ONLY")
		}
		if p.PrincipalId <= 0 {
			return errors.New("STATIC/PRINCIPAL 必须指定使用主体")
		}
		if p.CredentialPurposeId <= 0 {
			return errors.New("STATIC/PRINCIPAL 必须指定凭证用途")
		}
		if p.CallerId != "" {
			return errors.New("STATIC/PRINCIPAL 不允许配置 Caller")
		}
		if bindingCount != 0 {
			return errors.New("STATIC/PRINCIPAL 不允许绑定应用")
		}
	case p.IdentityMode == constant.IdentityModeStatic && p.AttributionTargetType == constant.AttributionTargetApplication:
		if p.IdentityAssurance != constant.IdentityAssuranceCredentialOnly {
			return errors.New("STATIC/APPLICATION 身份可信等级必须为 CREDENTIAL_ONLY")
		}
		if p.CallerId != "" {
			return errors.New("STATIC/APPLICATION 不允许配置 Caller")
		}
		if bindingCount != 1 {
			return errors.New("STATIC/APPLICATION 必须恰好绑定 1 个应用")
		}
	case p.IdentityMode == constant.IdentityModeDynamic && p.AttributionTargetType == constant.AttributionTargetPlatform:
		if p.IdentityAssurance != constant.IdentityAssuranceSignedContext {
			return errors.New("DYNAMIC/PLATFORM 身份可信等级必须为 SIGNED_CONTEXT")
		}
		if p.CallerId == "" {
			return errors.New("DYNAMIC/PLATFORM 必须配置 Caller")
		}
		if bindingCount < 1 {
			return errors.New("DYNAMIC/PLATFORM 至少绑定 1 个应用")
		}
		if p.PrincipalId != 0 {
			return errors.New("DYNAMIC/PLATFORM 第一阶段不允许指定使用主体")
		}
	case p.IdentityMode == constant.IdentityModeHybrid && p.AttributionTargetType == constant.AttributionTargetApplication:
		if p.IdentityAssurance != constant.IdentityAssuranceHybridVerified {
			return errors.New("HYBRID/APPLICATION 身份可信等级必须为 HYBRID_VERIFIED_CONTEXT")
		}
		if p.CallerId == "" {
			return errors.New("HYBRID/APPLICATION 必须配置 Caller")
		}
		if bindingCount != 1 {
			return errors.New("HYBRID/APPLICATION 必须恰好绑定 1 个应用")
		}
	default:
		return errors.New("非法的 identity_mode / attribution_target / identity_assurance 组合")
	}
	return nil
}

// validateProfileEnumFields 校验枚举字段是否合法。
func validateProfileEnumFields(p *model.AIIdentityProfile) error {
	if !constant.IdentityModeValid(p.IdentityMode) {
		return errors.New("身份取得方式 identity_mode 非法")
	}
	if !constant.AttributionTargetValid(p.AttributionTargetType) {
		return errors.New("归因目标 attribution_target_type 非法")
	}
	if !constant.IdentityAssuranceValid(p.IdentityAssurance) {
		return errors.New("身份可信等级 identity_assurance 非法")
	}
	return nil
}

// validateProfileRateLimit 校验 Profile 级限流参数（文档 6.8 通用规则）。
func validateProfileRateLimit(p *model.AIIdentityProfile) error {
	if !p.RateLimitEnabled {
		return nil
	}
	if p.RateLimitWindowSeconds < 10 || p.RateLimitWindowSeconds > 3600 {
		return errors.New("rate_limit_window_seconds 必须在 10 ~ 3600 之间")
	}
	if p.RateLimitMaxRequests < 1 || p.RateLimitMaxRequests > 100000 {
		return errors.New("rate_limit_max_requests 必须在 1 ~ 100000 之间")
	}
	return nil
}

// validateProfileReferences 校验 Profile 引用的使用主体/用途必须存在且启用。
// db 为事务句柄时，校验与后续写入在同一事务内完成（三库一致）。
func validateProfileReferences(db *gorm.DB, p *model.AIIdentityProfile) error {
	if p.PrincipalId > 0 {
		var principal model.AIPrincipal
		if err := db.First(&principal, p.PrincipalId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("使用主体不存在")
			}
			return err
		}
		if !principal.Enabled {
			return errors.New("已停用的使用主体不能被 Profile 引用")
		}
	}
	if p.CredentialPurposeId > 0 {
		var purpose model.AICredentialPurpose
		if err := db.First(&purpose, p.CredentialPurposeId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("凭证用途不存在")
			}
			return err
		}
		if !purpose.Enabled {
			return errors.New("已停用的凭证用途不能被 Profile 引用")
		}
	}
	return nil
}

// validateBindingsAppsEnabled 校验 Profile 绑定的应用存在且启用。
func validateBindingsAppsEnabled(db *gorm.DB, bindingCount int, appIDs []int) error {
	for _, appID := range appIDs {
		var app model.AIApplication
		if err := db.First(&app, appID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("绑定的应用不存在")
			}
			return err
		}
		if !app.Enabled {
			return errors.New("已停用的应用不能被 Profile 绑定")
		}
	}
	return nil
}

// checkDuplicateEnabledCredential 同 (principal, purpose, environment) 唯一 enabled 规则。
// 必须在事务内、锁定相同 principal 行后调用，以保证并发双启用被串行化。
func checkDuplicateEnabledCredential(db *gorm.DB, p *model.AIIdentityProfile) error {
	if p.IdentityMode != constant.IdentityModeStatic || p.AttributionTargetType != constant.AttributionTargetPrincipal {
		return nil
	}
	var count int64
	if err := db.Model(&model.AIIdentityProfile{}).
		Where("enabled = ? AND principal_id = ? AND credential_purpose_id = ? AND environment = ? AND id <> ?",
			true, p.PrincipalId, p.CredentialPurposeId, p.Environment, p.Id).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("同一使用主体 + 用途 + 环境 已存在启用中的个人弱身份 Profile，请先停用旧 Profile")
	}
	return nil
}

// countAppBindings 统计某 Profile 的绑定数量。
func countAppBindings(db *gorm.DB, profileID int) (int, error) {
	var count int64
	if err := db.Model(&model.AIIdentityAppBinding{}).Where("profile_id = ?", profileID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// profileHasActiveKey 判断某 Profile 是否存在 ACTIVE 状态签名密钥（仅看 status），
// 用于 generate/rotate 防止产生第二个 ACTIVE。注意它不代表“当前可用”。
func profileHasActiveKey(db *gorm.DB, profileID int) (bool, error) {
	var count int64
	if err := db.Model(&model.AIIdentitySigningKey{}).
		Where("profile_id = ? AND status = ?", profileID, constant.SigningKeyStatusActive).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// profileHasUsableActiveKey 判断某 Profile 是否存在“当前可用”的 ACTIVE 签名密钥，
// 用于 enable gate（6.10 规则 7）。条件：status ACTIVE、revoked_at=0、
// not_before<=now、expires_at=0 或 expires_at>now。
func profileHasUsableActiveKey(db *gorm.DB, profileID int) (bool, error) {
	now := common.GetTimestamp()
	var count int64
	if err := db.Model(&model.AIIdentitySigningKey{}).
		Where("profile_id = ? AND status = ? AND revoked_at = 0 AND not_before <= ? AND (expires_at = 0 OR expires_at > ?)",
			profileID, constant.SigningKeyStatusActive, now, now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// validateEnableProfile 校验一个 Profile 是否满足启用条件（绑定数以库内当前值为准）。
func validateEnableProfile(db *gorm.DB, p *model.AIIdentityProfile) error {
	return validateProfileEnable(db, p, true)
}

// validateProfileEnable 校验 Profile 是否处于合法可用状态（组合/引用/rate-limit/duplicate/app/key）。
//
// bindingCount 与已绑定应用取自数据库；requireKey 为 true 时，DYNAMIC/HYBRID 必须在
// 启用前存在 ACTIVE 签名密钥（6.10 规则 7）。CreateIdentityProfile 在创建即启用时
// 因尚无密钥而以 requireKey=false 校验，并另行拒绝 DYNAMIC/HYBRID 直接创建启用。
func validateProfileEnable(db *gorm.DB, p *model.AIIdentityProfile, requireKey bool) error {
	if err := validateProfileRateLimit(p); err != nil {
		return err
	}
	bindingCount, err := countAppBindings(db, p.Id)
	if err != nil {
		return err
	}
	if err := validateProfileCombination(p, bindingCount); err != nil {
		return err
	}
	if err := validateProfileReferences(db, p); err != nil {
		return err
	}
	if bindingCount > 0 {
		var appIDs []int
		if err := db.Model(&model.AIIdentityAppBinding{}).Where("profile_id = ?", p.Id).Pluck("app_id", &appIDs).Error; err != nil {
			return err
		}
		if err := validateBindingsAppsEnabled(db, bindingCount, appIDs); err != nil {
			return err
		}
	}
	if requireKey && (p.IdentityMode == constant.IdentityModeDynamic || p.IdentityMode == constant.IdentityModeHybrid) {
		hasKey, err := profileHasUsableActiveKey(db, p.Id)
		if err != nil {
			return err
		}
		if !hasKey {
			return errors.New(p.IdentityMode + " Profile 启用前必须存在当前可用 ACTIVE 签名密钥")
		}
	}
	if err := checkDuplicateEnabledCredential(db, p); err != nil {
		return err
	}
	return nil
}

// CreateIdentityProfile 创建 Identity Profile，并可选地附带初始 App 绑定。
// appIDs 为空表示不创建绑定；enabled=true 时执行完整启用门禁。
//
// 创建 Profile + App 绑定 + requested enabled 在单个事务内原子完成：任何一步失败
// 都整体回滚，不留半成品。appIDs 先去重再参与组合数量校验与写入。
func CreateIdentityProfile(p *model.AIIdentityProfile, appIDs []int) (*model.AIIdentityProfile, error) {
	if p.TokenId <= 0 {
		return nil, errors.New("必须指定 token_id")
	}
	if _, err := model.GetTokenById(p.TokenId); err != nil {
		return nil, errors.New("引用的 NewAPI Token 不存在")
	}
	// 输入规范化 + 纯校验（不依赖 DB 状态，可在事务外完成）。
	if err := validateProfileEnumFields(p); err != nil {
		return nil, err
	}
	p.CallerId = strings.TrimSpace(p.CallerId)
	p.CallerName = strings.TrimSpace(p.CallerName)
	p.Environment = strings.TrimSpace(p.Environment)
	if p.Environment == "" {
		p.Environment = constant.DefaultGovernanceEnvironment
	}
	if err := validateProfileTextField(utf8.RuneCountInString(p.CallerId), 128, "caller_id"); err != nil {
		return nil, err
	}
	if err := validateProfileTextField(utf8.RuneCountInString(p.CallerName), 128, "caller_name"); err != nil {
		return nil, err
	}
	if err := validateProfileTextField(utf8.RuneCountInString(p.Environment), 32, "environment"); err != nil {
		return nil, err
	}
	if p.RateLimitWindowSeconds == 0 {
		p.RateLimitWindowSeconds = 60
	}
	deduped := dedupAppIDs(appIDs)
	wantEnabled := p.Enabled
	// DYNAMIC/HYBRID 直接创建启用必须先禁用创建并生成密钥，第一阶段不支持。
	if wantEnabled && (p.IdentityMode == constant.IdentityModeDynamic || p.IdentityMode == constant.IdentityModeHybrid) {
		return nil, errors.New(p.IdentityMode + " Profile 须先创建（禁用）并生成签名密钥后再启用")
	}

	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	p.Enabled = wantEnabled

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		// 同一 token 仅一个 Profile（在事务内检查，避免并发绕过）。
		var existing int64
		if err := tx.Model(&model.AIIdentityProfile{}).Where("token_id = ?", p.TokenId).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return errors.New("同一 token_id 已存在 Identity Profile")
		}
		// 引用校验（使用 tx，避免 SQLite 单连接死锁）。
		if err := validateProfileReferences(tx, p); err != nil {
			return err
		}
		// 组合约束基于去重后的候选绑定数。
		if err := validateProfileCombination(p, len(deduped)); err != nil {
			return err
		}
		if len(deduped) > 0 {
			if err := validateBindingsAppsEnabled(tx, len(deduped), deduped); err != nil {
				return err
			}
		}
		if wantEnabled {
			if err := validateProfileRateLimit(p); err != nil {
				return err
			}
			// 以 principal 行为串行化键，锁后检查唯一 enabled 规则，避免并发双启用。
			if p.IdentityMode == constant.IdentityModeStatic && p.AttributionTargetType == constant.AttributionTargetPrincipal && p.PrincipalId > 0 {
				if _, err := model.LockAIPrincipal(tx, p.PrincipalId); err != nil {
					return err
				}
			}
			if err := checkDuplicateEnabledCredential(tx, p); err != nil {
				return err
			}
		}
		if err := tx.Create(p).Error; err != nil {
			return wrapUniqueError(err, "创建 Identity Profile 失败")
		}
		for _, appID := range deduped {
			b := &model.AIIdentityAppBinding{
				ProfileId: p.Id,
				AppId:     appID,
				Enabled:   true,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(b).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return p, nil
}

// IdentityProfilePatch 是 Profile 的补丁式更新输入。所有可选标量用指针，以区分
// “省略（保持原值）”与“显式 false / 0 / 空字符串”。
type IdentityProfilePatch struct {
	Id                     int
	IdentityMode           *string
	AttributionTargetType  *string
	IdentityAssurance      *string
	CallerId               *string
	CallerName             *string
	PrincipalId            *int
	CredentialPurposeId    *int
	Environment            *string
	RateLimitEnabled       *bool
	RateLimitWindowSeconds *int
	RateLimitMaxRequests   *int
	Enabled                *bool
}

// applyProfilePatch 将补丁字段应用到候选 Profile。字符串字段先 TrimSpace。
func applyProfilePatch(existing *model.AIIdentityProfile, patch *IdentityProfilePatch) {
	if patch.IdentityMode != nil {
		existing.IdentityMode = strings.TrimSpace(*patch.IdentityMode)
	}
	if patch.AttributionTargetType != nil {
		existing.AttributionTargetType = strings.TrimSpace(*patch.AttributionTargetType)
	}
	if patch.IdentityAssurance != nil {
		existing.IdentityAssurance = strings.TrimSpace(*patch.IdentityAssurance)
	}
	if patch.CallerId != nil {
		existing.CallerId = strings.TrimSpace(*patch.CallerId)
	}
	if patch.CallerName != nil {
		existing.CallerName = strings.TrimSpace(*patch.CallerName)
	}
	if patch.PrincipalId != nil {
		existing.PrincipalId = *patch.PrincipalId
	}
	if patch.CredentialPurposeId != nil {
		existing.CredentialPurposeId = *patch.CredentialPurposeId
	}
	if patch.Environment != nil {
		existing.Environment = strings.TrimSpace(*patch.Environment)
	}
	if patch.RateLimitEnabled != nil {
		existing.RateLimitEnabled = *patch.RateLimitEnabled
	}
	if patch.RateLimitWindowSeconds != nil {
		existing.RateLimitWindowSeconds = *patch.RateLimitWindowSeconds
	}
	if patch.RateLimitMaxRequests != nil {
		existing.RateLimitMaxRequests = *patch.RateLimitMaxRequests
	}
}

// buildProfileUpdates 依据现有与候选 Profile 的差异构造 SQL updates。
func buildProfileUpdates(existing, candidate *model.AIIdentityProfile) map[string]interface{} {
	updates := map[string]interface{}{"updated_at": common.GetTimestamp()}
	if candidate.IdentityMode != existing.IdentityMode {
		updates["identity_mode"] = candidate.IdentityMode
	}
	if candidate.AttributionTargetType != existing.AttributionTargetType {
		updates["attribution_target_type"] = candidate.AttributionTargetType
	}
	if candidate.IdentityAssurance != existing.IdentityAssurance {
		updates["identity_assurance"] = candidate.IdentityAssurance
	}
	if candidate.CallerId != existing.CallerId {
		updates["caller_id"] = candidate.CallerId
	}
	if candidate.CallerName != existing.CallerName {
		updates["caller_name"] = candidate.CallerName
	}
	if candidate.PrincipalId != existing.PrincipalId {
		updates["principal_id"] = candidate.PrincipalId
	}
	if candidate.CredentialPurposeId != existing.CredentialPurposeId {
		updates["credential_purpose_id"] = candidate.CredentialPurposeId
	}
	if candidate.Environment != existing.Environment {
		updates["environment"] = candidate.Environment
	}
	if candidate.RateLimitEnabled != existing.RateLimitEnabled {
		updates["rate_limit_enabled"] = candidate.RateLimitEnabled
	}
	if candidate.RateLimitWindowSeconds != existing.RateLimitWindowSeconds {
		updates["rate_limit_window_seconds"] = candidate.RateLimitWindowSeconds
	}
	if candidate.RateLimitMaxRequests != existing.RateLimitMaxRequests {
		updates["rate_limit_max_requests"] = candidate.RateLimitMaxRequests
	}
	return updates
}

// profileCoreTripletChanged 报告补丁是否改动身份核心三元组。
func profileCoreTripletChanged(patch *IdentityProfilePatch, existing *model.AIIdentityProfile) bool {
	if patch.IdentityMode != nil && strings.TrimSpace(*patch.IdentityMode) != existing.IdentityMode {
		return true
	}
	if patch.AttributionTargetType != nil && strings.TrimSpace(*patch.AttributionTargetType) != existing.AttributionTargetType {
		return true
	}
	if patch.IdentityAssurance != nil && strings.TrimSpace(*patch.IdentityAssurance) != existing.IdentityAssurance {
		return true
	}
	return false
}

// validateProfileTextField 校验 Profile 字符串字段长度（v 为字段的 Unicode 字符数）。
func validateProfileTextField(v, maxLen int, label string) error {
	if v > maxLen {
		return fmt.Errorf("%s 长度不能超过 %d 字符", label, maxLen)
	}
	return nil
}

// UpdateIdentityProfile 原子更新 Identity Profile。
//
// 语义：
//   - 采用补丁式输入（指针），省略字段保持原值，显式 false/0/空串按语义更新；
//   - 先构建完整候选对象并在事务前完成全部门禁校验，失败时 DB 完全不变；
//   - 已启用（或本次要启用）的 Profile 禁止修改核心三元组；
//   - 已启用 Profile 的任何字段更新都重新执行组合/引用/rate-limit/duplicate/app/key 门禁；
//   - 停用只置 enabled=false，无需完整合法性校验；
//   - 保持 disabled 的更新仍做结构/引用/组合一致性校验。
func UpdateIdentityProfile(patch *IdentityProfilePatch) (*model.AIIdentityProfile, error) {
	if patch == nil {
		return nil, errors.New("更新参数不能为空")
	}
	if patch.Id <= 0 {
		return nil, errors.New("id 非法")
	}

	var result *model.AIIdentityProfile
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		// 锁 Profile 行并重读当前态，串行化并发的 update/replace/密钥操作，避免
		// 校验与写入跨事务造成的竞态。
		existing, err := model.LockAIIdentityProfile(tx, patch.Id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("Identity Profile 不存在")
			}
			return err
		}

		targetEnabled := existing.Enabled
		if patch.Enabled != nil {
			targetEnabled = *patch.Enabled
		}

		// 已启用（或本次要启用）的 Profile 禁止修改核心三元组。
		if (existing.Enabled || targetEnabled) && profileCoreTripletChanged(patch, existing) {
			return errors.New("已启用的 Profile 禁止修改 identity_mode / attribution_target_type / identity_assurance")
		}

		// 显式空 environment 必须拒绝（存量 Environment 不允许为空）。
		if patch.Environment != nil && strings.TrimSpace(*patch.Environment) == "" {
			return errors.New("environment 不能为空")
		}

		// 构建完整候选对象并应用补丁字段。
		candidate := *existing
		applyProfilePatch(&candidate, patch)
		if err := validateProfileEnumFields(&candidate); err != nil {
			return err
		}
		if err := validateProfileTextField(utf8.RuneCountInString(candidate.CallerId), 128, "caller_id"); err != nil {
			return err
		}
		if err := validateProfileTextField(utf8.RuneCountInString(candidate.CallerName), 128, "caller_name"); err != nil {
			return err
		}
		if err := validateProfileTextField(utf8.RuneCountInString(candidate.Environment), 32, "environment"); err != nil {
			return err
		}

		// STATIC/PRINCIPAL 以 principal 行为串行化键，锁后检查唯一 enabled 规则，
		// 避免两个不同 token 的 Profile 并发双启用。
		if candidate.IdentityMode == constant.IdentityModeStatic && candidate.AttributionTargetType == constant.AttributionTargetPrincipal && candidate.PrincipalId > 0 {
			if _, err := model.LockAIPrincipal(tx, candidate.PrincipalId); err != nil {
				return err
			}
		}

		updates := buildProfileUpdates(existing, &candidate)

		if targetEnabled {
			// 启用（或已启用保持）：完整门禁，任一失败原子拒绝。
			if err := validateProfileEnable(tx, &candidate, true); err != nil {
				return err
			}
			if !existing.Enabled {
				updates["enabled"] = true
			}
		} else if existing.Enabled {
			// 停用：不要求完整合法性。
			updates["enabled"] = false
		} else {
			// 保持 disabled：只做结构/引用/组合一致性校验（不要求签名密钥）。
			bindingCount, err := countAppBindings(tx, existing.Id)
			if err != nil {
				return err
			}
			if err := validateProfileRateLimit(&candidate); err != nil {
				return err
			}
			if err := validateProfileCombination(&candidate, bindingCount); err != nil {
				return err
			}
			if err := validateProfileReferences(tx, &candidate); err != nil {
				return err
			}
			if bindingCount > 0 {
				var appIDs []int
				if err := tx.Model(&model.AIIdentityAppBinding{}).Where("profile_id = ?", existing.Id).Pluck("app_id", &appIDs).Error; err != nil {
					return err
				}
				if err := validateBindingsAppsEnabled(tx, bindingCount, appIDs); err != nil {
					return err
				}
			}
		}

		if err := tx.Model(&model.AIIdentityProfile{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
			return err
		}

		candidate.Enabled = targetEnabled
		candidate.UpdatedAt = common.GetTimestamp()
		result = &candidate
		return nil
	})
	if err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return result, nil
}

// DisableIdentityProfile 停用 Identity Profile（不自动停用 NewAPI Token）。
// 单事务内锁 Profile 行后更新，与并发的 update/replace/密钥操作串行化。
func DisableIdentityProfile(id int) error {
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		existing, err := model.LockAIIdentityProfile(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("Identity Profile 不存在")
			}
			return err
		}
		return tx.Model(existing).Updates(map[string]interface{}{
			"enabled":    false,
			"updated_at": common.GetTimestamp(),
		}).Error
	})
	if err != nil {
		return err
	}
	invalidateIdentitySnapshotCache()
	return nil
}

// GetIdentityProfile 查询单个 Profile。
func GetIdentityProfile(id int) (*model.AIIdentityProfile, error) {
	var p model.AIIdentityProfile
	if err := model.DB.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Identity Profile 不存在")
		}
		return nil, err
	}
	return &p, nil
}

func ListIdentityProfiles(page, pageSize int, keyword string, enabled *bool, identityMode string, tokenID int) ([]model.AIIdentityProfile, int64, error) {
	q := model.DB.Model(&model.AIIdentityProfile{})
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	if identityMode != "" {
		q = q.Where("identity_mode = ?", identityMode)
	}
	if tokenID > 0 {
		q = q.Where("token_id = ?", tokenID)
	}
	if keyword != "" {
		q = q.Where("(caller_id LIKE ? OR caller_name LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	return listPaged[model.AIIdentityProfile](q, page, pageSize)
}

// ListAppBindings 返回 Profile 的全部 App 绑定。
func ListAppBindings(profileID int) ([]model.AIIdentityAppBinding, error) {
	var bindings []model.AIIdentityAppBinding
	if err := model.DB.Where("profile_id = ?", profileID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// dedupAppIDs 去重并保持原顺序，避免重复 ID 依赖 DB 唯一约束产生半状态。
func dedupAppIDs(appIDs []int) []int {
	seen := make(map[int]struct{}, len(appIDs))
	out := make([]int, 0, len(appIDs))
	for _, id := range appIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// ReplaceAppBindings 原子替换 Profile 的应用绑定。
//
// 事务内先锁 Profile 行、重读当前态，再以同一 tx 完成绑定/组合/引用/rate-limit/
// duplicate/app/key 校验并写入；任何失败回滚，原绑定保持不变，且不会与并发的
// update/密钥操作竞态。appIDs 先去重再参与组合数量校验。
func ReplaceAppBindings(profileID int, appIDs []int) ([]model.AIIdentityAppBinding, error) {
	deduped := dedupAppIDs(appIDs)

	now := common.GetTimestamp()
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		p, err := model.LockAIIdentityProfile(tx, profileID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("Identity Profile 不存在")
			}
			return err
		}

		// STATIC/PRINCIPAL 以 principal 行为串行化键。
		if p.IdentityMode == constant.IdentityModeStatic && p.AttributionTargetType == constant.AttributionTargetPrincipal && p.PrincipalId > 0 {
			if _, err := model.LockAIPrincipal(tx, p.PrincipalId); err != nil {
				return err
			}
		}

		if p.Enabled {
			if err := validateProfileRateLimit(p); err != nil {
				return err
			}
			if err := validateProfileCombination(p, len(deduped)); err != nil {
				return err
			}
			if err := validateProfileReferences(tx, p); err != nil {
				return err
			}
			if len(deduped) > 0 {
				if err := validateBindingsAppsEnabled(tx, len(deduped), deduped); err != nil {
					return err
				}
			}
			if p.IdentityMode == constant.IdentityModeDynamic || p.IdentityMode == constant.IdentityModeHybrid {
				hasKey, err := profileHasActiveKey(tx, p.Id)
				if err != nil {
					return err
				}
				if !hasKey {
					return errors.New(p.IdentityMode + " Profile 启用前必须存在 ACTIVE 签名密钥")
				}
			}
			if err := checkDuplicateEnabledCredential(tx, p); err != nil {
				return err
			}
		} else if len(deduped) > 0 {
			// disabled Profile 仅校验绑定应用存在且启用。
			if err := validateBindingsAppsEnabled(tx, len(deduped), deduped); err != nil {
				return err
			}
		}

		if err := tx.Where("profile_id = ?", profileID).Delete(&model.AIIdentityAppBinding{}).Error; err != nil {
			return err
		}
		for _, appID := range deduped {
			b := &model.AIIdentityAppBinding{
				ProfileId: profileID,
				AppId:     appID,
				Enabled:   true,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(b).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	invalidateIdentitySnapshotCache()
	return ListAppBindings(profileID)
}

// ---------------------------------------------------------------------------
// 签名密钥管理
// ---------------------------------------------------------------------------

// ListSigningKeysMeta 返回某 Profile 的签名密钥元数据，绝不含明文/密文。
// 仅 SELECT 元数据列，不读取 secret_ciphertext。
func ListSigningKeysMeta(profileID int) ([]model.AIIdentitySigningKey, error) {
	var keys []model.AIIdentitySigningKey
	if err := model.DB.
		Select("id", "profile_id", "key_id", "status", "not_before", "expires_at", "revoked_at", "created_at", "updated_at").
		Where("profile_id = ?", profileID).Order("id asc").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// GetUsableSigningSecret 受控解密：仅返回可用签名密钥的原始字节，供运行时 HMAC 校验
// （第二批）使用，不暴露给 controller/API。校验 ACTIVE/RETIRING、not_before、expires_at、revoked。
func GetUsableSigningSecret(profileID int, keyID string) ([]byte, error) {
	key, err := GetSigningKey(profileID, keyID)
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	if key.Status != constant.SigningKeyStatusActive && key.Status != constant.SigningKeyStatusRetiring {
		return nil, errors.New("签名密钥状态不可用，仅 ACTIVE / RETIRING 可用于校验")
	}
	if key.RevokedAt != 0 {
		return nil, errors.New("签名密钥已撤销")
	}
	if key.NotBefore > now {
		return nil, errors.New("签名密钥尚未生效")
	}
	if key.ExpiresAt != 0 && key.ExpiresAt <= now {
		return nil, errors.New("签名密钥已过期")
	}
	return decryptSigningSecret(profileID, keyID, key.SecretCiphertext)
}

// GetSigningKey 查询单个签名密钥元数据。
func GetSigningKey(profileID int, keyID string) (*model.AIIdentitySigningKey, error) {
	var key model.AIIdentitySigningKey
	if err := model.DB.Where("profile_id = ? AND key_id = ?", profileID, keyID).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("签名密钥不存在")
		}
		return nil, err
	}
	return &key, nil
}

// requireSigningProfileTx 在事务内锁定 Profile 行并校验其身份模式为 DYNAMIC/HYBRID。
func requireSigningProfileTx(tx *gorm.DB, profileID int) (*model.AIIdentityProfile, error) {
	p, err := model.LockAIIdentityProfile(tx, profileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Identity Profile 不存在")
		}
		return nil, err
	}
	if p.IdentityMode != constant.IdentityModeDynamic && p.IdentityMode != constant.IdentityModeHybrid {
		return nil, errors.New("签名密钥只服务 DYNAMIC / HYBRID 身份 Profile")
	}
	return p, nil
}

// getSigningKeyInTx 在事务内查询单个签名密钥。
func getSigningKeyInTx(db *gorm.DB, profileID int, keyID string) (*model.AIIdentitySigningKey, error) {
	var key model.AIIdentitySigningKey
	if err := db.Where("profile_id = ? AND key_id = ?", profileID, keyID).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("签名密钥不存在")
		}
		return nil, err
	}
	return &key, nil
}

// GenerateSigningKey 生成第一个 ACTIVE 签名密钥。明文仅本次返回。
// 已存在 ACTIVE 密钥时拒绝，必须走 RotateSigningKey。
//
// 事务内锁 Profile 行、重读并检查“无 ACTIVE”后写入，保证并发下同一 Profile 至多
// 一个 ACTIVE（MySQL/PostgreSQL 由 Profile 行 FOR UPDATE 串行化，SQLite 由单写事务
// 兜底）。生成/加密在事务前完成以快速失败，不依赖 DB。
func GenerateSigningKey(profileID int) (*model.AIIdentitySigningKey, string, error) {
	keyID, err := GenerateKeyID()
	if err != nil {
		return nil, "", err
	}
	raw, display, err := GenerateSigningSecret()
	if err != nil {
		return nil, "", err
	}
	ciphertext, err := encryptSigningSecret(profileID, keyID, raw)
	if err != nil {
		return nil, "", err
	}
	now := common.GetTimestamp()
	key := &model.AIIdentitySigningKey{
		ProfileId:        profileID,
		KeyId:            keyID,
		SecretCiphertext: ciphertext,
		Status:           constant.SigningKeyStatusActive,
		NotBefore:        now,
		ExpiresAt:        0,
		RevokedAt:        0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := requireSigningProfileTx(tx, profileID); err != nil {
			return err
		}
		hasActive, err := profileHasActiveKey(tx, profileID)
		if err != nil {
			return err
		}
		if hasActive {
			return errors.New("已存在 ACTIVE 签名密钥，如需更换请使用 rotate")
		}
		return tx.Create(key).Error
	})
	if err != nil {
		return nil, "", err
	}
	invalidateIdentitySnapshotCache()
	return key, display, nil
}

// RotateSigningKey 原子轮换签名密钥：旧 ACTIVE 置为 RETIRING（含 24h 宽限期），生成新 ACTIVE。
// 无当前 ACTIVE 时拒绝。生成/加密在事务前完成，任何失败回滚，旧 ACTIVE 保持不变；明文仅本次返回。
func RotateSigningKey(profileID int) (*model.AIIdentitySigningKey, string, error) {
	keyID, err := GenerateKeyID()
	if err != nil {
		return nil, "", err
	}
	raw, display, err := GenerateSigningSecret()
	if err != nil {
		return nil, "", err
	}
	ciphertext, err := encryptSigningSecret(profileID, keyID, raw)
	if err != nil {
		return nil, "", err
	}

	now := common.GetTimestamp()
	newKey := &model.AIIdentitySigningKey{
		ProfileId:        profileID,
		KeyId:            keyID,
		SecretCiphertext: ciphertext,
		Status:           constant.SigningKeyStatusActive,
		NotBefore:        now,
		ExpiresAt:        0,
		RevokedAt:        0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := requireSigningProfileTx(tx, profileID); err != nil {
			return err
		}
		// 无当前 ACTIVE 不得 rotate。
		hasActive, err := profileHasActiveKey(tx, profileID)
		if err != nil {
			return err
		}
		if !hasActive {
			return errors.New("无 ACTIVE 签名密钥，无法轮换，请先 generate")
		}
		// 旧 ACTIVE → RETIRING，并设置 24h 宽限期。
		if err := tx.Model(&model.AIIdentitySigningKey{}).
			Where("profile_id = ? AND status = ?", profileID, constant.SigningKeyStatusActive).
			Updates(map[string]interface{}{
				"status":     constant.SigningKeyStatusRetiring,
				"expires_at": now + constant.SigningKeyGracePeriodSeconds,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		// 创建新 ACTIVE。
		return tx.Create(newKey).Error
	})
	if err != nil {
		return nil, "", err
	}
	invalidateIdentitySnapshotCache()
	return newKey, display, nil
}

// RevokeSigningKey 撤销签名密钥。REVOKED 不允许恢复。key_id 上限 64 字符。
func RevokeSigningKey(profileID int, keyID string) error {
	if utf8.RuneCountInString(keyID) > 64 {
		return errors.New("密钥编号长度不能超过 64 字符")
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := model.LockAIIdentityProfile(tx, profileID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("Identity Profile 不存在")
			}
			return err
		}
		key, err := getSigningKeyInTx(tx, profileID, keyID)
		if err != nil {
			return err
		}
		if key.Status == constant.SigningKeyStatusRevoked {
			return errors.New("密钥已撤销，不允许恢复")
		}
		now := common.GetTimestamp()
		return tx.Model(key).
			Updates(map[string]interface{}{"status": constant.SigningKeyStatusRevoked, "revoked_at": now, "updated_at": now}).Error
	})
	if err != nil {
		return err
	}
	invalidateIdentitySnapshotCache()
	return nil
}

// ---------------------------------------------------------------------------
// 分页辅助
// ---------------------------------------------------------------------------

type pagedModel interface {
	model.AIBusinessDomain | model.AIOwnerTeam | model.AIUsageTeam | model.AIPrincipal |
		model.AICredentialPurpose | model.AIApplication | model.AIIdentityProfile
}

func listPaged[T pagedModel](q *gorm.DB, page, pageSize int) ([]T, int64, error) {
	var items []T
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if err := q.Offset((page - 1) * pageSize).Limit(pageSize).Order("id asc").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// wrapUniqueError 将唯一约束冲突转换为友好的业务错误。
func wrapUniqueError(err error, msg string) error {
	if err == nil {
		return nil
	}
	if isUniqueConstraintError(err) {
		return errors.New(msg)
	}
	return err
}

func isUniqueConstraintError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "unique") || strings.Contains(s, "Duplicate entry") ||
		strings.Contains(s, "duplicate key") || strings.Contains(s, "UNIQUE constraint failed")
}
