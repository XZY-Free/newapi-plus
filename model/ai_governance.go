package model

// 第一批企业身份归因主数据 / 凭证治理 / 签名密钥 / 审计事件模型。
//
// 这 10 张表仅进入主库 DB（同时注册于 migrateDB 与 migrateDBFast），
// 绝不进入 LOG_DB / ClickHouse。字段、索引、唯一约束、TableName 与
// 三数据库兼容严格按文档第 6.2 ~ 6.12 节执行。

import (
	"errors"

	"gorm.io/gorm"
)

// AIBusinessDomain 业务领域主数据（ai_business_domains）。
type AIBusinessDomain struct {
	Id         int    `json:"id" gorm:"primaryKey"`
	DomainCode string `json:"domain_code" gorm:"type:varchar(64);uniqueIndex"`
	DomainName string `json:"domain_name" gorm:"type:varchar(128)"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

func (AIBusinessDomain) TableName() string { return "ai_business_domains" }

// AIOwnerTeam AI 应用建设/维护/运营负责团队（ai_owner_teams）。
// 注意：该表表示“负责建设应用的团队”，不是 Key 使用人的所属团队。
type AIOwnerTeam struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	TeamCode  string `json:"team_code" gorm:"type:varchar(64);uniqueIndex"`
	TeamName  string `json:"team_name" gorm:"type:varchar(128)"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (AIOwnerTeam) TableName() string { return "ai_owner_teams" }

// AIUsageTeam 凭证使用人的组织/使用团队（ai_usage_teams）。
type AIUsageTeam struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	TeamCode  string `json:"team_code" gorm:"type:varchar(64);uniqueIndex"`
	TeamName  string `json:"team_name" gorm:"type:varchar(128)"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (AIUsageTeam) TableName() string { return "ai_usage_teams" }

// AIPrincipal 个人弱身份凭证的责任主体/使用主体（ai_principals）。
// 第一阶段只支持 PERSON，不建设企业人员目录同步。
type AIPrincipal struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	PrincipalCode    string `json:"principal_code" gorm:"type:varchar(128);uniqueIndex"`
	PrincipalName    string `json:"principal_name" gorm:"type:varchar(128)"`
	PrincipalType    string `json:"principal_type" gorm:"type:varchar(16)"`
	BusinessDomainID int    `json:"business_domain_id" gorm:"index"`
	UsageTeamID      int    `json:"usage_team_id" gorm:"index"`
	Enabled          bool   `json:"enabled"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (AIPrincipal) TableName() string { return "ai_principals" }

// AICredentialPurpose 公司批准某把个人/固定 Key 用于什么场景（ai_credential_purposes）。
// 它是“登记用途”，不是“已验证客户端”。
type AICredentialPurpose struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	PurposeCode string `json:"purpose_code" gorm:"type:varchar(64);uniqueIndex"`
	PurposeName string `json:"purpose_name" gorm:"type:varchar(128)"`
	PurposeType string `json:"purpose_type" gorm:"type:varchar(32)"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (AICredentialPurpose) TableName() string { return "ai_credential_purposes" }

// AIApplication 企业 AI 应用（ai_applications）。
// 对外协议字段 root_app_id 使用稳定 app_code；数据库内部自增 id 仅用于关联和索引。
type AIApplication struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	AppCode          string `json:"app_code" gorm:"type:varchar(64);uniqueIndex"`
	AppName          string `json:"app_name" gorm:"type:varchar(128)"`
	BusinessDomainID int    `json:"business_domain_id" gorm:"index"`
	OwnerTeamID      int    `json:"owner_team_id" gorm:"index"`
	Enabled          bool   `json:"enabled"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (AIApplication) TableName() string { return "ai_applications" }

// AIIdentityProfile 把 NewAPI Token 转换为企业治理凭证配置（ai_identity_profiles）。
// 组合约束（STATIC/PRINCIPAL、STATIC/APPLICATION、DYNAMIC/PLATFORM、HYBRID/APPLICATION）
// 必须由 service 层强制，见 service/ai_identity.go。
type AIIdentityProfile struct {
	Id                     int    `json:"id" gorm:"primaryKey"`
	TokenId                int    `json:"token_id" gorm:"uniqueIndex"`
	IdentityMode           string `json:"identity_mode" gorm:"type:varchar(16)"`
	AttributionTargetType  string `json:"attribution_target_type" gorm:"type:varchar(16)"`
	IdentityAssurance      string `json:"identity_assurance" gorm:"type:varchar(32)"`
	CallerId               string `json:"caller_id" gorm:"type:varchar(128)"`
	CallerName             string `json:"caller_name" gorm:"type:varchar(128)"`
	PrincipalId            int    `json:"principal_id" gorm:"index"`
	CredentialPurposeId    int    `json:"credential_purpose_id" gorm:"index"`
	Environment            string `json:"environment" gorm:"type:varchar(32)"`
	RateLimitEnabled       bool   `json:"rate_limit_enabled"`
	RateLimitWindowSeconds int    `json:"rate_limit_window_seconds"`
	RateLimitMaxRequests   int    `json:"rate_limit_max_requests"`
	Enabled                bool   `json:"enabled"`
	CreatedAt              int64  `json:"created_at"`
	UpdatedAt              int64  `json:"updated_at"`
}

func (AIIdentityProfile) TableName() string { return "ai_identity_profiles" }

// AIIdentityAppBinding 身份配置与应用绑定（ai_identity_app_bindings）。
// 唯一约束：profile_id + app_id。
type AIIdentityAppBinding struct {
	Id        int   `json:"id" gorm:"primaryKey"`
	ProfileId int   `json:"profile_id" gorm:"index;uniqueIndex:uniq_profile_app,priority:1"`
	AppId     int   `json:"app_id" gorm:"index;uniqueIndex:uniq_profile_app,priority:2"`
	Enabled   bool  `json:"enabled"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

func (AIIdentityAppBinding) TableName() string { return "ai_identity_app_bindings" }

// AIIdentitySigningKey 只服务 DYNAMIC / HYBRID 的签名密钥（ai_identity_signing_keys）。
// secret_ciphertext 只存带版本前缀密文，任何查询 API 永不返回明文或密文。
type AIIdentitySigningKey struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	ProfileId        int    `json:"profile_id" gorm:"index;uniqueIndex:uniq_profile_key,priority:1"`
	KeyId            string `json:"key_id" gorm:"type:varchar(64);uniqueIndex:uniq_profile_key,priority:2"`
	SecretCiphertext string `json:"-" gorm:"type:text"`
	Status           string `json:"status" gorm:"type:varchar(16)"`
	NotBefore        int64  `json:"not_before"`
	ExpiresAt        int64  `json:"expires_at"`
	RevokedAt        int64  `json:"revoked_at"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (AIIdentitySigningKey) TableName() string { return "ai_identity_signing_keys" }

// AIIdentityAuditEvent 身份验证失败/降级审计（ai_identity_audit_events）。
// 严禁保存 API Key、Signing Secret、Signature、原始 Context、Nonce、Prompt、Response。
type AIIdentityAuditEvent struct {
	Id                  int    `json:"id" gorm:"primaryKey"`
	CreatedAt           int64  `json:"created_at" gorm:"index"`
	RequestId           string `json:"request_id" gorm:"type:varchar(64);index"`
	TokenId             int    `json:"token_id" gorm:"index"`
	ProfileId           int    `json:"profile_id" gorm:"index"`
	CallerId            string `json:"caller_id" gorm:"type:varchar(128)"`
	PrincipalId         int    `json:"principal_id" gorm:"index"`
	CredentialPurposeId int    `json:"credential_purpose_id" gorm:"index"`
	IdentityMode        string `json:"identity_mode" gorm:"type:varchar(16)"`
	IdentityAssurance   string `json:"identity_assurance" gorm:"type:varchar(32)"`
	Result              string `json:"result" gorm:"type:varchar(16)"`
	ReasonCode          string `json:"reason_code" gorm:"type:varchar(64);index"`
	ClaimedRootAppId    string `json:"claimed_root_app_id" gorm:"type:varchar(128)"`
	HttpMethod          string `json:"http_method" gorm:"type:varchar(16)"`
	RequestPath         string `json:"request_path" gorm:"type:varchar(256)"`
	ClientIp            string `json:"client_ip" gorm:"type:varchar(64)"`
}

func (AIIdentityAuditEvent) TableName() string { return "ai_identity_audit_events" }

// LockAIIdentityProfile 在事务内按 id 锁定 Identity Profile 行（FOR UPDATE；SQLite 走
// lockForUpdate 的兼容路径，其单写模型由事务串行化兜底）。所有对 Profile 的
// 读改写必须经由本方法，以保证并发 generate/rotate/revoke/update 串行化。
func LockAIIdentityProfile(tx *gorm.DB, id int) (*AIIdentityProfile, error) {
	if id <= 0 {
		return nil, errors.New("id 非法")
	}
	var p AIIdentityProfile
	if err := lockForUpdate(tx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// LockAIPrincipal 在事务内锁定使用主体行，作为 STATIC/PRINCIPAL 唯一启用规则
// （同一 principal+purpose+environment 仅一个 enabled Profile）的串行化键，避免
// 两个不同 token 的 Profile 并发双启用。
func LockAIPrincipal(tx *gorm.DB, id int) (*AIPrincipal, error) {
	if id <= 0 {
		return nil, errors.New("id 非法")
	}
	var p AIPrincipal
	if err := lockForUpdate(tx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// GetAIBusinessApplication 按 id 查询 AI 应用。
func GetAIBusinessApplication(id int) (*AIApplication, error) {
	if id <= 0 {
		return nil, errors.New("id 非法")
	}
	var app AIApplication
	if err := DB.First(&app, id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// aiGovernanceModels 返回第一批全部主库治理模型，供 migrateDB / migrateDBFast 共用。
func AIGovernanceModels() []interface{} {
	return []interface{}{
		&AIBusinessDomain{},
		&AIOwnerTeam{},
		&AIUsageTeam{},
		&AIPrincipal{},
		&AICredentialPurpose{},
		&AIApplication{},
		&AIIdentityProfile{},
		&AIIdentityAppBinding{},
		&AIIdentitySigningKey{},
		&AIIdentityAuditEvent{},
		// 第六批：企业用量整点投影（§12）
		&AIUsageHourly{},
	}
}
