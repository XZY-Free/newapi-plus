package constant

// AI Attribution / Identity Governance 第一批常量。
//
// 本文件承载第一阶段“企业身份归因、弱身份凭证治理”所需的常量定义与
// Gin Context 承载的“可信归因上下文”类型。
//
// 说明：文档建议将 Trusted Context 放在 types/ai_attribution.go，但现有
// types 包已 import common（types/rw_map.go），而 common/ai_attribution_context.go
// 需要承载该类型，会导致 common -> types -> common 循环依赖。故该类型放入 constant
// （common 与 types 都已依赖 constant），快照/DTO 类型仍留在 types。
//
// 所有定义与
// docs/chatgpt/NewAPI_企业身份归因弱身份凭证治理与OpenTelemetry_第一阶段详细改造方案_V1.1.md
// 第 6 章保持一致。

// TrustedAttributionContext 是在请求生命周期内于 Gin Context 中传递的“可信归因上下文”。
//
// 第一批只完成该类型的定义与 Gin Context 的 Set/Get 基础（common/ai_attribution_context.go），
// 不挂载运行时中间件。字段语义与文档第 7.13 节保持一致。
type TrustedAttributionContext struct {
	// TokenID 是 NewAPI Token 的技术身份锚点，由 TokenAuth 认证后写入。
	TokenID int `json:"token_id"`

	// ProfileID 是 ai_identity_profiles 的主键；未登记时为 0。
	ProfileID int `json:"profile_id"`

	// CredentialVerified 表示该请求使用的 API Key 已被 TokenAuth 验证。
	CredentialVerified bool `json:"credential_verified"`
	// ClientVerified 表示客户端/平台本身是否被密码学验证。
	// 对 CREDENTIAL_ONLY 永远为 false，不得由 User-Agent 等推断为 true。
	ClientVerified bool `json:"client_verified"`

	IdentityMode      string `json:"identity_mode"`
	AttributionTarget string `json:"attribution_target"`
	IdentityAssurance string `json:"identity_assurance"`

	// 弱身份个人归因
	PrincipalID           int    `json:"principal_id"`
	PrincipalCode         string `json:"principal_code"`
	CredentialPurposeID   int    `json:"credential_purpose_id"`
	CredentialPurposeCode string `json:"credential_purpose_code"`
	Environment           string `json:"environment"`

	// 强身份/应用归因
	CallerID   string `json:"caller_id"`
	CallerName string `json:"caller_name"`
	RootAppID  string `json:"root_app_id"` // 使用稳定 app_code
	// ApplicationID 是 ai_applications 的自增 id，仅用于关联与索引。
	ApplicationID int `json:"application_id"`

	// SignedContext 标志本次请求是否携带了有效的签名执行上下文。
	SignedContext bool `json:"signed_context"`
}

// CredentialOnly 报告该可信等级是否仅为“凭证已验证”。
func (t *TrustedAttributionContext) CredentialOnly() bool {
	return t.IdentityAssurance == IdentityAssuranceCredentialOnly
}

// 身份取得方式（identity_mode）。
const (
	IdentityModeStatic  = "STATIC"
	IdentityModeDynamic = "DYNAMIC"
	IdentityModeHybrid  = "HYBRID"
)

// 归因目标（attribution_target_type）。
const (
	AttributionTargetPrincipal   = "PRINCIPAL"
	AttributionTargetApplication = "APPLICATION"
	AttributionTargetPlatform    = "PLATFORM"
)

// 身份可信等级（identity_assurance）。
// 注意：身份取得方式（STATIC/DYNAMIC/HYBRID）与可信等级（CREDENTIAL_ONLY/...）
// 是两个正交维度，不得混为一个字段。
const (
	IdentityAssuranceCredentialOnly = "CREDENTIAL_ONLY"
	IdentityAssuranceSignedContext  = "SIGNED_CONTEXT"
	IdentityAssuranceHybridVerified = "HYBRID_VERIFIED_CONTEXT"
)

// 弱身份凭证安全姿态风险等级。
const (
	RiskLower  = "LOWER_RISK"
	RiskMedium = "MEDIUM_RISK"
	RiskHigh   = "HIGH_RISK"
)

// 使用主体类型（principal_type）。第一阶段只支持 PERSON。
const (
	PrincipalTypePerson = "PERSON"
)

// 凭证登记用途类型（purpose_type）。
const (
	PurposeTypeDesktopClient = "DESKTOP_CLIENT"
	PurposeTypeIDE           = "IDE"
	PurposeTypeScript        = "SCRIPT"
	PurposeTypeService       = "SERVICE"
	PurposeTypeOther         = "OTHER"
)

// 签名密钥状态（status）。
const (
	SigningKeyStatusActive   = "ACTIVE"
	SigningKeyStatusRetiring = "RETIRING"
	SigningKeyStatusRevoked  = "REVOKED"
)

// 治理环境默认值。
const (
	DefaultGovernanceEnvironment = "prod"
)

// 身份审计结果。
const (
	IdentityAuditResultUnverified = "UNVERIFIED"
	IdentityAuditResultRejected   = "REJECTED"
)

// 环境变量名。
const (
	AIAttributionMasterKeyEnv       = "AI_ATTRIBUTION_MASTER_KEY"
	AICredentialRotationDaysEnv     = "AI_CREDENTIAL_ROTATION_DAYS"
	AICredentialRotationDaysMin     = 30
	AICredentialRotationDaysMax     = 365
	AICredentialRotationDaysDefault = 90
)

// Signing Secret 存储版本前缀。数据库密文以该前缀开头，便于未来升级算法。
const (
	SigningSecretVersionPrefix = "v1:"
	SigningSecretMasterKeyLen  = 32
	SigningSecretRawLen        = 32
	SigningSecretNonceLen      = 12
)

// SigningKeyGracePeriodSeconds 是轮换后旧签名密钥的默认宽限期：24 小时。
// 轮换时将旧 ACTIVE 的 expires_at 置为 now+24h。
const SigningKeyGracePeriodSeconds = 24 * 3600

// 第一阶段确定性风险/审计 reason_code。
const (
	ReasonCodeCredentialRateLimitExceeded         = "CREDENTIAL_RATE_LIMIT_EXCEEDED"
	ReasonCodeCredentialRateLimitStoreUnavailable = "CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE"
	ReasonCodeCredentialRotationOverdue           = "CREDENTIAL_ROTATION_OVERDUE"
	ReasonCodeCredentialHighRisk                  = "CREDENTIAL_HIGH_RISK"
)

// IdentityModeValid 校验身份取得方式是否合法。
func IdentityModeValid(mode string) bool {
	switch mode {
	case IdentityModeStatic, IdentityModeDynamic, IdentityModeHybrid:
		return true
	}
	return false
}

// AttributionTargetValid 校验归因目标是否合法。
func AttributionTargetValid(target string) bool {
	switch target {
	case AttributionTargetPrincipal, AttributionTargetApplication, AttributionTargetPlatform:
		return true
	}
	return false
}

// IdentityAssuranceValid 校验身份可信等级是否合法。
func IdentityAssuranceValid(assurance string) bool {
	switch assurance {
	case IdentityAssuranceCredentialOnly, IdentityAssuranceSignedContext, IdentityAssuranceHybridVerified:
		return true
	}
	return false
}

// PurposeTypeValid 校验凭证用途类型是否合法。
func PurposeTypeValid(t string) bool {
	switch t {
	case PurposeTypeDesktopClient, PurposeTypeIDE, PurposeTypeScript, PurposeTypeService, PurposeTypeOther:
		return true
	}
	return false
}

// SigningKeyStatusValid 校验签名密钥状态是否合法。
func SigningKeyStatusValid(status string) bool {
	switch status {
	case SigningKeyStatusActive, SigningKeyStatusRetiring, SigningKeyStatusRevoked:
		return true
	}
	return false
}
