package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// AIUsageHourly 企业用量整点投影（ai_usage_hourly，V1.1 §12）。
//
// 事实来源仍是 Consume/Error/Task Billing Log 的 Other.ai_attribution 快照，
// 本表只是查询优化。每请求/事件归一到整点桶，按 dimension_hash 唯一聚合并原子
// 替换目标时间范围（§12.6）。绝不进入 LOG_DB/ClickHouse，仅主库 DB。
type AIUsageHourly struct {
	Id int64 `json:"id" gorm:"primaryKey;AUTO_INCREMENT"`

	BucketTime int64 `json:"bucket_time" gorm:"index"` // 整点 Unix 秒

	// --- 维度（§12.2） ---
	ProfileID            int    `json:"profile_id"`             // Identity Profile，未归因为 0
	PrincipalID          int    `json:"principal_id"`           // 弱身份个人主体，无则 0
	CredentialPurposeID  int    `json:"credential_purpose_id"`  // 弱身份用途，无则 0
	UsageBusinessDomainID int   `json:"usage_business_domain_id"` // 人员所属领域，无则 0
	UsageTeamID          int    `json:"usage_team_id"`          // 使用团队，无则 0
	CallerKey            string `json:"caller_key"`             // 强身份 Caller 稳定短字符串；弱身份为空
	RootAppCode          string `json:"root_app_code"`          // root_app_id 稳定 app_code；无则空
	AppID                int    `json:"app_id"`                 // 内部 ai_applications.id（按 app_code 尽力解析，未知为 0）
	AppBusinessDomainID  int    `json:"app_business_domain_id"` // 应用领域
	OwnerTeamID          int    `json:"owner_team_id"`          // 应用建设团队
	IdentityAssurance    string `json:"identity_assurance"`     // CREDENTIAL_ONLY / SIGNED_CONTEXT / HYBRID_VERIFIED_CONTEXT / UNVERIFIED
	ClientVerified       bool   `json:"client_verified"`        // §12.3：仅 client_verified=true 的 DYNAMIC/HYBRID 计入正式 Caller/App 统计
	ModelName            string `json:"model_name"`

	DimensionHash string `json:"dimension_hash" gorm:"type:varchar(64);uniqueIndex"` // 稳定维度哈希（§12.2）

	// --- 度量 ---
	RequestCount     int64 `json:"request_count"`
	SuccessCount     int64 `json:"success_count"`
	ErrorCount       int64 `json:"error_count"`
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	QuotaNet         int64 `json:"quota_net"`         // 净 Quota（bigint）
	DurationMsTotal  int64 `json:"duration_ms_total"` // 总耗时

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

func (AIUsageHourly) TableName() string { return "ai_usage_hourly" }

// UsageProjectionDim 是投影一行所需的维度与度量输入，由调用方从 Log.ai_attribution 快照
// 与请求指标填充，再经 AIUsageHourlyRow() 派生稳定 dimension_hash。
type UsageProjectionDim struct {
	BucketTime            int64
	ProfileID             int
	PrincipalID           int
	CredentialPurposeID   int
	UsageBusinessDomainID int
	UsageTeamID           int
	CallerKey             string
	RootAppCode           string
	AppID                 int
	AppBusinessDomainID   int
	OwnerTeamID           int
	IdentityAssurance     string
	ClientVerified        bool
	ModelName             string
}

// CanonicalDimension 生成 §12.2 唯一聚合维度的规范字符串。
// 顺序固定：bucket + profile + principal + purpose + usage_domain + usage_team +
// caller_key + app_code + app_domain + owner_team + assurance + model_name。
// 任何调用方不得自行增减字段，否则 dimension_hash 会漂移。
func (d UsageProjectionDim) CanonicalDimension() string {
	assurance := d.IdentityAssurance
	if assurance == "" {
		assurance = "UNVERIFIED"
	}
	return strings.Join([]string{
		strconv.FormatInt(d.BucketTime, 10),
		strconv.Itoa(d.ProfileID),
		strconv.Itoa(d.PrincipalID),
		strconv.Itoa(d.CredentialPurposeID),
		strconv.Itoa(d.UsageBusinessDomainID),
		strconv.Itoa(d.UsageTeamID),
		d.CallerKey,
		d.RootAppCode,
		strconv.Itoa(d.AppID),
		strconv.Itoa(d.AppBusinessDomainID),
		strconv.Itoa(d.OwnerTeamID),
		assurance,
		d.ModelName,
	}, "|")
}

// HashDimension 返回规范维度的稳定 SHA-256 十六进制串（§12.2 dimension_hash）。
func HashDimension(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// AIUsageHourlyRow 由维度输入构造一行（含 dimension_hash），供 Upsert 前使用。
func AIUsageHourlyRow(dim UsageProjectionDim) AIUsageHourly {
	canonical := dim.CanonicalDimension()
	return AIUsageHourly{
		BucketTime:             dim.BucketTime,
		ProfileID:              dim.ProfileID,
		PrincipalID:            dim.PrincipalID,
		CredentialPurposeID:    dim.CredentialPurposeID,
		UsageBusinessDomainID:  dim.UsageBusinessDomainID,
		UsageTeamID:            dim.UsageTeamID,
		CallerKey:              dim.CallerKey,
		RootAppCode:            dim.RootAppCode,
		AppID:                  dim.AppID,
		AppBusinessDomainID:    dim.AppBusinessDomainID,
		OwnerTeamID:            dim.OwnerTeamID,
		IdentityAssurance:      dim.CanonicalAssurance(),
		ClientVerified:         dim.ClientVerified,
		ModelName:              dim.ModelName,
		DimensionHash:          HashDimension(canonical),
	}
}

// CanonicalAssurance 规范化 identity_assurance 空值（§12.2 用 UNVERIFIED）。
func (d UsageProjectionDim) CanonicalAssurance() string {
	if d.IdentityAssurance == "" {
		return "UNVERIFIED"
	}
	return d.IdentityAssurance
}

// UpsertUsageHourly 将一行累加到对应 dimension_hash 桶（主库 DB，原子 upsert）。
// 不存在则插入，存在则累加度量与请求计数。
func UpsertUsageHourly(row *AIUsageHourly) error {
	if row.DimensionHash == "" {
		row.DimensionHash = AIUsageHourlyRow(UsageProjectionDim{
			BucketTime: row.BucketTime, ProfileID: row.ProfileID, PrincipalID: row.PrincipalID,
			CredentialPurposeID: row.CredentialPurposeID, UsageBusinessDomainID: row.UsageBusinessDomainID,
			UsageTeamID: row.UsageTeamID, CallerKey: row.CallerKey, RootAppCode: row.RootAppCode,
			AppID: row.AppID, AppBusinessDomainID: row.AppBusinessDomainID, OwnerTeamID: row.OwnerTeamID,
			IdentityAssurance: row.IdentityAssurance, ClientVerified: row.ClientVerified, ModelName: row.ModelName,
		}).DimensionHash
	}
	// 保持原样，重复调用会累加（重建时先原子替换范围，见 ReplaceUsageProjectionRange）。
	return DB.Transaction(func(tx *gorm.DB) error {
		var existing AIUsageHourly
		err := tx.Where("dimension_hash = ?", row.DimensionHash).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			row.CreatedAt = common.GetTimestamp()
			row.UpdatedAt = row.CreatedAt
			return tx.Create(row).Error
		}
		if err != nil {
			return err
		}
		updates := map[string]any{
			"request_count":     existing.RequestCount + row.RequestCount,
			"success_count":     existing.SuccessCount + row.SuccessCount,
			"error_count":       existing.ErrorCount + row.ErrorCount,
			"input_tokens":      existing.InputTokens + row.InputTokens,
			"output_tokens":     existing.OutputTokens + row.OutputTokens,
			"total_tokens":      existing.TotalTokens + row.TotalTokens,
			"quota_net":         existing.QuotaNet + row.QuotaNet,
			"duration_ms_total": existing.DurationMsTotal + row.DurationMsTotal,
			"updated_at":        common.GetTimestamp(),
		}
		return tx.Model(&AIUsageHourly{}).Where("id = ?", existing.Id).Updates(updates).Error
	})
}

// ReplaceUsageProjectionRange 原子替换 [bucketStart, bucketEnd]（整点闭区间）内的投影
// 数据：先删后插于同一事务，失败不留半清空状态（§12.6 幂等/原子语义）。
func ReplaceUsageProjectionRange(bucketStart, bucketEnd int64, rows []*AIUsageHourly) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bucket_time >= ? AND bucket_time <= ?", bucketStart, bucketEnd).
			Delete(&AIUsageHourly{}).Error; err != nil {
			return err
		}
		for _, r := range rows {
			r.CreatedAt = common.GetTimestamp()
			r.UpdatedAt = r.CreatedAt
			if err := tx.Create(r).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteUsageProjectionRange 删除某整点范围内投影（重建失败时清场或按需回滚）。
func DeleteUsageProjectionRange(bucketStart, bucketEnd int64) error {
	return DB.Where("bucket_time >= ? AND bucket_time <= ?", bucketStart, bucketEnd).
		Delete(&AIUsageHourly{}).Error
}

// UsageProjectionFilter 企业统计 API 的筛选条件（§12.7）。
type UsageProjectionFilter struct {
	BucketStart           int64
	BucketEnd             int64
	ProfileID             int
	PrincipalID           int
	CredentialPurposeID   int
	UsageBusinessDomainID int
	UsageTeamID           int
	CallerKey             string
	RootAppCode           string
	AppBusinessDomainID   int
	OwnerTeamID           int
	IdentityAssurance     string
	ModelName             string
}

// buildQuery 将筛选条件转为 GORM 查询。
func (f UsageProjectionFilter) buildQuery(tx *gorm.DB) *gorm.DB {
	if f.BucketStart != 0 {
		tx = tx.Where("bucket_time >= ?", f.BucketStart)
	}
	if f.BucketEnd != 0 {
		tx = tx.Where("bucket_time <= ?", f.BucketEnd)
	}
	if f.ProfileID != 0 {
		tx = tx.Where("profile_id = ?", f.ProfileID)
	}
	if f.PrincipalID != 0 {
		tx = tx.Where("principal_id = ?", f.PrincipalID)
	}
	if f.CredentialPurposeID != 0 {
		tx = tx.Where("credential_purpose_id = ?", f.CredentialPurposeID)
	}
	if f.UsageBusinessDomainID != 0 {
		tx = tx.Where("usage_business_domain_id = ?", f.UsageBusinessDomainID)
	}
	if f.UsageTeamID != 0 {
		tx = tx.Where("usage_team_id = ?", f.UsageTeamID)
	}
	if f.CallerKey != "" {
		tx = tx.Where("caller_key = ?", f.CallerKey)
	}
	if f.RootAppCode != "" {
		tx = tx.Where("root_app_code = ?", f.RootAppCode)
	}
	if f.AppBusinessDomainID != 0 {
		tx = tx.Where("app_business_domain_id = ?", f.AppBusinessDomainID)
	}
	if f.OwnerTeamID != 0 {
		tx = tx.Where("owner_team_id = ?", f.OwnerTeamID)
	}
	if f.IdentityAssurance != "" {
		tx = tx.Where("identity_assurance = ?", f.IdentityAssurance)
	}
	if f.ModelName != "" {
		tx = tx.Where("model_name = ?", f.ModelName)
	}
	return tx
}

// QueryUsageRows 按筛选条件返回投影明细（§12.7 统计 API 底层）。
func QueryUsageRows(f UsageProjectionFilter) ([]*AIUsageHourly, error) {
	var rows []*AIUsageHourly
	tx := DB.Model(&AIUsageHourly{})
	tx = f.buildQuery(tx)
	err := tx.Order("bucket_time asc").Find(&rows).Error
	return rows, err
}

// ParseUsageAttribution 从 ai_attribution 快照 map 解析投影维度。
// 返回是否解析出任何可信归因（否则走未归因桶）。
func ParseUsageAttribution(a map[string]interface{}) (UsageProjectionDim, bool) {
	var d UsageProjectionDim
	if a == nil {
		return d, false
	}
	d.ProfileID = intFromMap(a, "profile_id")
	d.PrincipalID = intFromMap(a, "principal_id")
	d.CredentialPurposeID = intFromMap(a, "credential_purpose_id")
	d.UsageBusinessDomainID = intFromMap(a, "usage_business_domain_id")
	d.UsageTeamID = intFromMap(a, "usage_team_id")
	d.CallerKey = strFromMap(a, "caller_id")
	d.RootAppCode = strFromMap(a, "root_app_id")
	d.AppBusinessDomainID = intFromMap(a, "application_business_domain_id")
	d.OwnerTeamID = intFromMap(a, "owner_team_id")
	d.IdentityAssurance = strFromMap(a, "identity_assurance")
	d.ClientVerified = boolFromMap(a, "client_verified")
	// 强身份 CALLER 未验证仍可能带 caller_id，但 §12.3 要求仅 client_verified=true
	// 才计入正式 Caller/App 统计；未验证进入“未验证”桶。
	hasAttribution := d.ProfileID != 0 || d.PrincipalID != 0 || d.CallerKey != "" || d.RootAppCode != "" ||
		d.CredentialPurposeID != 0 || d.UsageTeamID != 0 || d.UsageBusinessDomainID != 0
	return d, hasAttribution
}

func intFromMap(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}

func strFromMap(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

func boolFromMap(m map[string]interface{}, key string) bool {
	b, _ := m[key].(bool)
	return b
}

// ResolveAppIDFromCode 尽力将 app_code 解析为内部 ai_applications.id（未知或 DB 不可用返回 0）。
func ResolveAppIDFromCode(code string) int {
	if code == "" || DB == nil {
		return 0
	}
	var app AIApplication
	err := DB.Where("app_code = ?", code).First(&app).Error
	if err != nil {
		return 0
	}
	return app.Id
}
