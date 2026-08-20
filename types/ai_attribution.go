// Package types 承载第一阶段企业身份归因所需的、不依赖数据库访问的类型。
//
// 这些类型只表达“身份事实”与“归因快照”，不持有 GORM 模型，也不直接访问数据库，
// 因此可被 service / middleware / common / controller 安全引用而不会引入循环依赖。
package types

// SnapshotApplication 描述 Identity Snapshot 中一个已绑定且启用的应用及其快照。
type SnapshotApplication struct {
	ApplicationID int    `json:"application_id"`
	AppCode       string `json:"app_code"`
	AppName       string `json:"app_name"`
	AppEnabled    bool   `json:"app_enabled"`

	BusinessDomainID   int    `json:"business_domain_id"`
	BusinessDomainCode string `json:"business_domain_code"`
	BusinessDomainName string `json:"business_domain_name"`

	OwnerTeamID   int    `json:"owner_team_id"`
	OwnerTeamCode string `json:"owner_team_code"`
	OwnerTeamName string `json:"owner_team_name"`

	BindingEnabled bool `json:"binding_enabled"`
}

// SignedExecutionContext 是 X-AI-Context 解码后的协议字段（文档 7.3）。
// 严格 Schema：仅允许下列字段，未知字段拒绝；其余身份事实一律由网关从主数据决定。
type SignedExecutionContext struct {
	RootAppID          string `json:"root_app_id"`
	RootRunID          string `json:"root_run_id"`
	CurrentExecutionID string `json:"current_execution_id"`
	ParentExecutionID  string `json:"parent_execution_id"`
	ExecutionType      string `json:"execution_type"`
	ExecutionDepth     *int   `json:"execution_depth"`
	WorkflowID         string `json:"workflow_id"`
	AgentID            string `json:"agent_id"`
	TaskID             string `json:"task_id"`
	NodeID             string `json:"node_id"`
}

// ProfileRateLimit 是企业 Profile 级请求频率限制配置。
type ProfileRateLimit struct {
	Enabled       bool `json:"enabled"`
	WindowSeconds int  `json:"window_seconds"`
	MaxRequests   int  `json:"max_requests"`
}

// CredentialSecurity 是 NewAPI Token 现有安全配置的摘要（用于风险姿态）。
// 绝不包含 Token Key 明文。
type CredentialSecurity struct {
	IPRestricted     bool  `json:"ip_restricted"`
	ModelRestricted  bool  `json:"model_restricted"`
	QuotaRestricted  bool  `json:"quota_restricted"`
	ExpiryConfigured bool  `json:"expiry_configured"`
	UnlimitedQuota   bool  `json:"unlimited_quota"`
	CreatedTime      int64 `json:"created_time"`
	ExpiredTime      int64 `json:"expired_time"`
}

// RiskPosture 是弱身份凭证安全姿态（文档 6.15.2）。绝不包含 Token Key 明文。
type RiskPosture struct {
	IPRestricted        bool   `json:"ip_restricted"`
	ModelRestricted     bool   `json:"model_restricted"`
	QuotaRestricted     bool   `json:"quota_restricted"`
	ExpiryConfigured    bool   `json:"expiry_configured"`
	RateLimitEnabled    bool   `json:"rate_limit_enabled"`
	CredentialOnly      bool   `json:"credential_only"`
	RotationOverdue     bool   `json:"rotation_overdue"`
	RotationOverdueDays int64  `json:"rotation_overdue_days,omitempty"`
	RiskLevel           string `json:"risk_level"`
}

// IdentitySnapshot 是按 token_id 统一读取的完整身份快照。
// 第二批运行时中间件不得自行拼多表查询，必须通过该快照取得全部身份事实。
type IdentitySnapshot struct {
	ProfileID         int    `json:"profile_id"`
	TokenID           int    `json:"token_id"`
	Enabled           bool   `json:"enabled"`
	IdentityMode      string `json:"identity_mode"`
	AttributionTarget string `json:"attribution_target"`
	IdentityAssurance string `json:"identity_assurance"`
	Environment       string `json:"environment"`
	CallerID          string `json:"caller_id"`
	CallerName        string `json:"caller_name"`

	PrincipalID              int    `json:"principal_id"`
	PrincipalCode            string `json:"principal_code"`
	PrincipalName            string `json:"principal_name"`
	PrincipalEnabled         bool   `json:"principal_enabled"`
	UsageDomainID            int    `json:"usage_domain_id"`
	UsageDomainCode          string `json:"usage_domain_code"`
	UsageDomainName          string `json:"usage_domain_name"`
	UsageTeamID              int    `json:"usage_team_id"`
	UsageTeamCode            string `json:"usage_team_code"`
	UsageTeamName            string `json:"usage_team_name"`
	CredentialPurposeID      int    `json:"credential_purpose_id"`
	CredentialPurposeCode    string `json:"credential_purpose_code"`
	CredentialPurposeName    string `json:"credential_purpose_name"`
	CredentialPurposeEnabled bool   `json:"credential_purpose_enabled"`

	Applications []SnapshotApplication `json:"applications"`

	RateLimit ProfileRateLimit `json:"rate_limit"`

	// HasActiveSigningKey 表示该 Profile 是否存在可用的 ACTIVE 签名密钥。
	HasActiveSigningKey bool `json:"has_active_signing_key"`

	// SigningKeys 是可用的签名密钥元数据（key_id/status/not_before/expires_at/revoked_at）。
	// 绝不含 ciphertext / plaintext。不含 REVOKED 密钥。
	SigningKeys []SigningKeyMeta `json:"signing_keys"`

	TokenSecurity *CredentialSecurity `json:"token_security,omitempty"`
}

// SigningKeyMeta 是签名密钥的元数据视图，供快照与列表使用。绝不含密文/明文。
type SigningKeyMeta struct {
	KeyId     string `json:"key_id"`
	Status    string `json:"status"`
	NotBefore int64  `json:"not_before"`
	ExpiresAt int64  `json:"expires_at"`
	RevokedAt int64  `json:"revoked_at"`
}

// Clone 返回快照的深拷贝。调用者修改返回的快照不得污染缓存内部对象。
func (s *IdentitySnapshot) Clone() *IdentitySnapshot {
	if s == nil {
		return nil
	}
	c := *s
	if s.Applications != nil {
		c.Applications = make([]SnapshotApplication, len(s.Applications))
		copy(c.Applications, s.Applications)
	}
	if s.SigningKeys != nil {
		c.SigningKeys = make([]SigningKeyMeta, len(s.SigningKeys))
		copy(c.SigningKeys, s.SigningKeys)
	}
	if s.TokenSecurity != nil {
		cs := *s.TokenSecurity
		c.TokenSecurity = &cs
	}
	return &c
}
