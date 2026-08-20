package service

import (
	"encoding/base64"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }

// createStaticPrincipalProfile 创建 STATIC/PRINCIPAL Profile（disabled 或 enabled）。
func createStaticPrincipalProfile(t *testing.T, tokenID, principalID, purposeID int, enabled bool) *model.AIIdentityProfile {
	t.Helper()
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           principalID, CredentialPurposeId: purposeID,
		Environment: "prod", Enabled: enabled,
	}, nil)
	require.NoError(t, err)
	return p
}

// createDynamicPlatformProfile 创建 DYNAMIC/PLATFORM disabled Profile（可选绑定应用）。
func createDynamicPlatformProfile(t *testing.T, tokenID, appID int) *model.AIIdentityProfile {
	t.Helper()
	appIDs := []int{}
	if appID > 0 {
		appIDs = []int{appID}
	}
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "workflow-prod",
	}, appIDs)
	require.NoError(t, err)
	return p
}

func signingKeysForProfile(t *testing.T, profileID int) []model.AIIdentitySigningKey {
	t.Helper()
	keys, err := ListSigningKeysMeta(profileID)
	require.NoError(t, err)
	return keys
}

func activeKeyCount(t *testing.T, profileID int) int {
	t.Helper()
	keys := signingKeysForProfile(t, profileID)
	n := 0
	for _, k := range keys {
		if k.Status == constant.SigningKeyStatusActive {
			n++
		}
	}
	return n
}

// 问题 2：无关 Profile PATCH 不得改变任何其他字段。
func TestIdentityUpdateUnrelatedFieldLeavesOthersIntact(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	rateEnabled := true
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID,
		Environment: "prod", RateLimitEnabled: true,
		RateLimitWindowSeconds: 60, RateLimitMaxRequests: 100,
	}, nil)
	require.NoError(t, err)

	// 只改 caller_name（STATIC/PRINCIPAL 不限制 caller_name）。
	updated, err := UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, CallerName: strPtr("新名字")})
	require.NoError(t, err)
	assert.Equal(t, "新名字", updated.CallerName)
	assert.Equal(t, f.PrincipalID, updated.PrincipalId, "principal_id 不得被无关更新改变")
	assert.Equal(t, f.PurposeID, updated.CredentialPurposeId, "credential_purpose_id 不得被无关更新改变")
	assert.Equal(t, "prod", updated.Environment, "environment 不得被无关更新改变")
	assert.Equal(t, rateEnabled, updated.RateLimitEnabled, "rate_limit_enabled 不得被无关更新改变")
	assert.Equal(t, 60, updated.RateLimitWindowSeconds, "rate_limit_window 不得被无关更新改变")
	assert.Equal(t, 100, updated.RateLimitMaxRequests, "rate_limit_max 不得被无关更新改变")
}

// 问题 2：显式 false / 0 / 空串按语义更新（disabled DYNAMIC/PLATFORM 上允许清空 caller_name / 关闭限流）。
func TestIdentityUpdateExplicitZeroFalseEmptyApply(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	rateEnabled := true
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "workflow-prod", CallerName: "旧名字",
		RateLimitEnabled: true, RateLimitWindowSeconds: 60, RateLimitMaxRequests: 100,
	}, []int{f.AppID})
	require.NoError(t, err)

	// 显式置空 caller_name、关闭限流、window=0。
	window := 0
	updated, err := UpdateIdentityProfile(&IdentityProfilePatch{
		Id: p.Id, CallerName: strPtr(""), RateLimitEnabled: boolPtr(false),
		RateLimitWindowSeconds: &window,
	})
	require.NoError(t, err)
	assert.Equal(t, "", updated.CallerName, "显式空串必须按语义清空")
	assert.False(t, updated.RateLimitEnabled, "显式 false 必须生效")
	assert.Equal(t, 0, updated.RateLimitWindowSeconds, "显式 0 必须生效")
	assert.Equal(t, "workflow-prod", updated.CallerId, "caller_id 未被显式清空必须保留")
	_ = rateEnabled

	// 校验落库。
	stored, err := GetIdentityProfile(p.Id)
	require.NoError(t, err)
	assert.Equal(t, "", stored.CallerName)
	assert.False(t, stored.RateLimitEnabled)
	assert.Equal(t, 0, stored.RateLimitWindowSeconds)
}

// 问题 3：disabled Profile 引用更新被拒绝且 DB 不变。
func TestIdentityDisabledProfileRefUpdateRejectedDBUnchanged(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createStaticPrincipalProfile(t, tokenID, f.PrincipalID, f.PurposeID, false)

	// 第二个 disabled principal。
	p2, err := CreatePrincipal("lisi", "李四", f.DomainID, f.UsageTeamID)
	require.NoError(t, err)
	disable := false
	UpdatePrincipal(p2.Id, "", 0, 0, &disable)

	// 更新引用到已停用的 principal → 拒绝。
	_, err = UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, PrincipalId: intPtr(p2.Id)})
	require.Error(t, err, "引用已停用的使用主体必须被拒绝")

	// DB 不变。
	stored, err := GetIdentityProfile(p.Id)
	require.NoError(t, err)
	assert.Equal(t, f.PrincipalID, stored.PrincipalId, "失败更新不得改变 principal_id")
}

// 问题 3：启用门禁失败时 DB 不变（enabled 保持 false）。
func TestIdentityFailedEnableLeavesDBUnchanged(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	// 非法 rate_limit（window=5）在 disabled 创建时允许，但启用会失败。
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           f.PrincipalID, CredentialPurposeId: f.PurposeID,
		Environment: "prod", RateLimitEnabled: true,
		RateLimitWindowSeconds: 5, RateLimitMaxRequests: 100,
	}, nil)
	require.NoError(t, err)
	require.False(t, p.Enabled)

	_, err = UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, Enabled: boolPtr(true)})
	require.Error(t, err, "非法 rate_limit 的启用必须被拒绝")

	stored, err := GetIdentityProfile(p.Id)
	require.NoError(t, err)
	assert.False(t, stored.Enabled, "启用失败后 enabled 必须保持 false")
}

// 问题 3：enabled Profile 非法更新被原子拒绝且 DB 不变。
func TestIdentityEnabledProfileInvalidUpdateDBUnchanged(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createStaticPrincipalProfile(t, tokenID, f.PrincipalID, f.PurposeID, true)
	require.True(t, p.Enabled)

	// 已启用 Profile 上打开限流但 max=0 → 非法 → 原子拒绝。
	_, err := UpdateIdentityProfile(&IdentityProfilePatch{
		Id: p.Id, RateLimitEnabled: boolPtr(true), RateLimitMaxRequests: intPtr(0),
	})
	require.Error(t, err)

	stored, err := GetIdentityProfile(p.Id)
	require.NoError(t, err)
	assert.False(t, stored.RateLimitEnabled, "失败更新不得打开 rate_limit_enabled")
}

// 问题 4：绑定替换失败时保留原绑定。
func TestIdentityFailedBindingReplacementKeepsOriginal(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	// STATIC/APPLICATION 恰好 1 绑定。
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetApplication,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		Enabled:               true,
	}, []int{f.AppID})
	require.NoError(t, err)
	require.True(t, p.Enabled)

	// 替换为 2 个应用 → 组合失败。
	_, err = ReplaceAppBindings(p.Id, []int{f.AppID, f.App2ID})
	require.Error(t, err, "enabled STATIC/APPLICATION 必须恰好 1 个绑定")

	bindings, err := ListAppBindings(p.Id)
	require.NoError(t, err)
	require.Len(t, bindings, 1, "失败替换必须保留原绑定")
	assert.Equal(t, f.AppID, bindings[0].AppId)
}

// 问题 5：初始 generate 只允许无 ACTIVE 时；第二次 generate 拒绝。
func TestIdentitySecondGenerateRejected(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)

	_, secret, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)
	require.NotEmpty(t, secret)
	require.Equal(t, 1, activeKeyCount(t, p.Id))

	_, _, err = GenerateSigningKey(p.Id)
	require.Error(t, err, "已有 ACTIVE 时第二次 generate 必须拒绝")
	require.Equal(t, 1, activeKeyCount(t, p.Id), "拒绝后仍只有 1 个 ACTIVE")
}

// 问题 5：rotate 在加密失败（错误 master key）时保持原 ACTIVE，DB 不变。
func TestIdentityRotateFailureKeepsOriginalActive(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	key1, _, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	// 换成错误长度的 master key → rotate 加密失败，必须在事务前返回。
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 16)))
	_, _, err = RotateSigningKey(p.Id)
	require.Error(t, err, "错误 master key 必须使 rotate 失败")

	// 原 ACTIVE 保持不变，且没有新增密钥。
	keys := signingKeysForProfile(t, p.Id)
	require.Len(t, keys, 1, "rotate 失败不得新增密钥")
	assert.Equal(t, constant.SigningKeyStatusActive, keys[0].Status, "rotate 失败原密钥必须保持 ACTIVE")
	assert.Equal(t, key1.KeyId, keys[0].KeyId)
}

// 问题 5：rotate 原子地产出恰好一个 ACTIVE、旧 Key RETIRING 且带 24h 宽限期。
func TestIdentityRotateAtomicOneActiveOldRetiringWithGrace(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	key1, _, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	key2, secret2, err := RotateSigningKey(p.Id)
	require.NoError(t, err)
	require.NotEmpty(t, secret2)

	keys := signingKeysForProfile(t, p.Id)
	require.Len(t, keys, 2)
	require.Equal(t, 1, activeKeyCount(t, p.Id), "必须恰好 1 个 ACTIVE")

	statusByID := map[string]string{}
	expiryByID := map[string]int64{}
	notBeforeByID := map[string]int64{}
	for _, k := range keys {
		statusByID[k.KeyId] = k.Status
		expiryByID[k.KeyId] = k.ExpiresAt
		notBeforeByID[k.KeyId] = k.NotBefore
	}
	assert.Equal(t, constant.SigningKeyStatusRetiring, statusByID[key1.KeyId], "旧 Key 必须进入 RETIRING")
	assert.Equal(t, constant.SigningKeyStatusActive, statusByID[key2.KeyId], "新 Key 必须 ACTIVE")
	assert.Equal(t, int64(0), expiryByID[key2.KeyId], "新 Key 永不过期")
	// 旧 Key 宽限期：expires_at >= not_before + 24h。
	assert.GreaterOrEqual(t, expiryByID[key1.KeyId], notBeforeByID[key1.KeyId]+constant.SigningKeyGracePeriodSeconds,
		"旧 Key 必须设置至少 24h 宽限期")
	assert.LessOrEqual(t, expiryByID[key1.KeyId], notBeforeByID[key1.KeyId]+constant.SigningKeyGracePeriodSeconds+10,
		"宽限期不应超过 24h（加少量时钟误差）")
}

// 问题 6：快照返回深拷贝，调用者修改不得污染缓存。
func TestIdentitySnapshotCopyMutationIsolation(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	_, _, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	snap, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.Len(t, snap.SigningKeys, 1)
	require.True(t, snap.HasActiveSigningKey)

	// 修改返回的快照（污染尝试）。
	snap.SigningKeys = append(snap.SigningKeys, types.SigningKeyMeta{KeyId: "dummy", Status: "ACTIVE"})

	// 再次读取应返回干净的深拷贝。
	snap2, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.Len(t, snap2.SigningKeys, 1, "缓存必须返回深拷贝，修改不得污染缓存")
	require.True(t, snap2.HasActiveSigningKey)
}

// 问题 6：快照引用表缺失必须返回错误，不得静默半快照。
func TestIdentitySnapshotMissingRefReturnsError(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	p := createStaticPrincipalProfile(t, tokenID, f.PrincipalID, f.PurposeID, false)

	// 删除被引用的 principal 行。
	require.NoError(t, model.DB.Delete(&model.AIPrincipal{}, f.PrincipalID).Error)

	_, err := GetIdentitySnapshotByTokenID(tokenID)
	require.Error(t, err, "引用表缺失必须返回错误")
	_ = p
}

// 问题 6：可用签名密钥元数据与受控解密。
func TestIdentitySigningKeyMetaAndControlledDecryption(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	key, secret, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	// 快照签名密钥元数据：含 key_id/status，不含密文字段。
	snap, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.Len(t, snap.SigningKeys, 1)
	assert.Equal(t, key.KeyId, snap.SigningKeys[0].KeyId)
	assert.Equal(t, constant.SigningKeyStatusActive, snap.SigningKeys[0].Status)

	// 受控解密：ACTIVE 可用。
	raw, err := GetUsableSigningSecret(p.Id, key.KeyId)
	require.NoError(t, err)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(raw), secret)

	// 撤销后解密拒绝。
	require.NoError(t, RevokeSigningKey(p.Id, key.KeyId))
	_, err = GetUsableSigningSecret(p.Id, key.KeyId)
	require.Error(t, err, "已撤销密钥不得用于校验")
}

// 审查问题 7：密钥 expires_at == now 即视为不可用。
func TestIdentityUsableSecretExpiresAtNowRejected(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	key, _, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.AIIdentitySigningKey{}).
		Where("profile_id = ? AND key_id = ?", p.Id, key.KeyId).
		Update("expires_at", now).Error)

	_, err = GetUsableSigningSecret(p.Id, key.KeyId)
	require.Error(t, err, "到期等于 now 时必须不可用")
}

// 审查问题：rotate 无当前 ACTIVE 必须拒绝（当前实现会静默创建一个 ACTIVE）。
func TestIdentityRotateWithoutActiveRejected(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	_, _, err := RotateSigningKey(p.Id)
	require.Error(t, err, "无 ACTIVE 签名密钥不得 rotate")
	require.Equal(t, 0, activeKeyCount(t, p.Id))
}

// 审查问题：主数据名称为 Unicode 字符数上限 128（非字节数）。
func TestIdentityNameUnicodeBoundary(t *testing.T) {
	setupAIGovernanceService(t)
	good := strings.Repeat("中", 128)
	_, err := CreateBusinessDomain("d128", good)
	require.NoError(t, err, "128 个 Unicode 字符应通过 varchar(128) 校验")

	bad := strings.Repeat("中", 129)
	_, err = CreateBusinessDomain("d129", bad)
	require.Error(t, err, "129 个 Unicode 字符应被拒绝")
}

// 审查问题：主数据 update 空白名称必须拒绝，不能 Trim 后写空。
func TestIdentityUpdateWhitespaceNameRejected(t *testing.T) {
	setupAIGovernanceService(t)
	d, err := CreateBusinessDomain("finance", "财务")
	require.NoError(t, err)
	enabled := true
	_, err = UpdateBusinessDomain(d.Id, "   ", &enabled)
	require.Error(t, err, "空白名称 update 必须拒绝")
}

// 审查问题：Update 显式 environment="" 必须拒绝。
func TestIdentityUpdateEmptyEnvironmentRejected(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	p := createStaticPrincipalProfile(t, tokenID, f.PrincipalID, f.PurposeID, false)
	_, err := UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, Environment: strPtr("")})
	require.Error(t, err, "显式空 environment 必须拒绝")
}

// 审查问题：创建时重复 appIDs 去重后不影响组合数量（STATIC/APPLICATION 恰好 1 绑定）。
func TestIdentityCreateDupAppIDsDedupCombination(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetApplication,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		Enabled:               false,
	}, []int{f.AppID, f.AppID})
	require.NoError(t, err, "重复 appIDs 去重后应通过 STATIC/APPLICATION 恰好 1 绑定")
	bindings, err := ListAppBindings(p.Id)
	require.NoError(t, err)
	require.Len(t, bindings, 1, "去重后应恰好 1 个绑定")
}

// 审查问题：创建即启用失败必须不留任何 Profile / Binding 残留。
func TestIdentityCreateEnableAtomicNoPartial(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	// 引用已停用的 principal → 启用校验失败。
	p2, err := CreatePrincipal("lisi", "李四", f.DomainID, f.UsageTeamID)
	require.NoError(t, err)
	disable := false
	_, err = UpdatePrincipal(p2.Id, "", 0, 0, &disable)
	require.NoError(t, err)

	_, err = CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		PrincipalId:           p2.Id, CredentialPurposeId: f.PurposeID,
		Enabled: true,
	}, nil)
	require.Error(t, err)

	var cnt int64
	require.NoError(t, model.DB.Model(&model.AIIdentityProfile{}).Where("token_id = ?", tokenID).Count(&cnt).Error)
	require.Equal(t, int64(0), cnt, "创建失败不得残留 Profile")
}

// 审查问题：并发 generate 必须保证最终最多一个 ACTIVE。
func TestIdentityConcurrentGenerateAtMostOneActive(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	const n = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := GenerateSigningKey(p.Id); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	require.GreaterOrEqual(t, successes, 1, "至少一个 generate 成功")
	require.Equal(t, 1, activeKeyCount(t, p.Id), "并发下最终必须恰好 1 个 ACTIVE")
}

// 审查问题：revoke 的 key_id 上限为 64（不是 128）。
func TestIdentityRevokeKeyIDTooLongRejected(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	_, _, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	err = RevokeSigningKey(p.Id, strings.Repeat("k", 65))
	require.Error(t, err, "key_id 超过 64 字符必须拒绝")
}

// 审查问题 1：enable gate 要求“当前可用 ACTIVE”（status ACTIVE、revoked_at=0、
// not_before<=now、expires_at=0 或 >now）；已到期/未来生效 ACTIVE 不能启用。
func TestIdentityEnableRequiresUsableActive(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	key, _, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	now := common.GetTimestamp()
	// 已到期 ACTIVE → 不能启用。
	require.NoError(t, model.DB.Model(&model.AIIdentitySigningKey{}).
		Where("profile_id = ? AND key_id = ?", p.Id, key.KeyId).
		Update("expires_at", now).Error)
	_, err = UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, Enabled: boolPtr(true)})
	require.Error(t, err, "已到期 ACTIVE 不能启用")
	stored, err := GetIdentityProfile(p.Id)
	require.NoError(t, err)
	require.False(t, stored.Enabled, "启用失败后必须保持 disabled")

	// 恢复未到期 → 可启用。
	require.NoError(t, model.DB.Model(&model.AIIdentitySigningKey{}).
		Where("profile_id = ? AND key_id = ?", p.Id, key.KeyId).
		Update("expires_at", 0).Error)
	_, err = UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, Enabled: boolPtr(true)})
	require.NoError(t, err, "当前可用 ACTIVE 应可启用")

	// 未来生效 ACTIVE → 不能启用。
	require.NoError(t, DisableIdentityProfile(p.Id))
	require.NoError(t, model.DB.Model(&model.AIIdentitySigningKey{}).
		Where("profile_id = ? AND key_id = ?", p.Id, key.KeyId).
		Update("not_before", now+3600).Error)
	_, err = UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, Enabled: boolPtr(true)})
	require.Error(t, err, "未来生效 ACTIVE 不能启用")
}

// 审查问题 2：快照只包含当前可用于验证的 ACTIVE/RETIRING；未来/到期/REVOKED 一律不入快照。
func TestIdentitySnapshotExcludesUnusableKeys(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	key1, _, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)
	key2, _, err := RotateSigningKey(p.Id)
	require.NoError(t, err)

	snap, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.Len(t, snap.SigningKeys, 2, "ACTIVE + 宽限期内 RETIRING 都应进入快照")
	require.True(t, snap.HasActiveSigningKey)

	now := common.GetTimestamp()
	// 令 ACTIVE(key2) 到期 → 不再可用；旧 RETIRING(key1) 仍在宽限期内可用。
	require.NoError(t, model.DB.Model(&model.AIIdentitySigningKey{}).
		Where("profile_id = ? AND key_id = ?", p.Id, key2.KeyId).
		Update("expires_at", now).Error)
	invalidateIdentitySnapshotCache()
	snap2, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.False(t, snap2.HasActiveSigningKey, "已到期 ACTIVE 不得计入 HasActiveSigningKey")
	require.Len(t, snap2.SigningKeys, 1, "仅宽限期内 RETIRING 进入快照")
	require.Equal(t, key1.KeyId, snap2.SigningKeys[0].KeyId)

	// 令 RETIRING(key1) 撤销 → 快照为空。
	require.NoError(t, model.DB.Model(&model.AIIdentitySigningKey{}).
		Where("profile_id = ? AND key_id = ?", p.Id, key1.KeyId).
		Update("revoked_at", now).Error)
	invalidateIdentitySnapshotCache()
	snap3, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.Empty(t, snap3.SigningKeys, "撤销的密钥不得进入快照")
	require.False(t, snap3.HasActiveSigningKey)
}

func TestIdentitySnapshotSigningKeyJSONNoCiphertext(t *testing.T) {
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)

	p := createDynamicPlatformProfile(t, tokenID, f.AppID)
	_, _, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)

	snap, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	j, err := common.Marshal(snap)
	require.NoError(t, err)
	require.NotContains(t, string(j), "secret_ciphertext", "快照 JSON 不得包含密文字段")
	require.NotContains(t, string(j), "ciphertext", "快照 JSON 不得泄露密文")
	require.Equal(t, p.Id, snap.ProfileID)
}
