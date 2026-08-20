package constant

import "strings"

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
// 第二批补齐文档 7.13 节全部字段。运行时就地生成并写入 Gin Context，随请求流转；
// 绝不得携带 Signing Secret / Signature / Nonce / 原始 Context / API Key 明文。
type TrustedAttributionContext struct {
	// --- 凭证事实（7.13 凭证事实） ---
	TokenID            int    `json:"token_id"`
	ProfileID          int    `json:"profile_id"`
	CredentialVerified bool   `json:"credential_verified"`
	Environment        string `json:"environment"`

	// --- 身份模式与可信等级（7.13） ---
	IdentityMode      string `json:"identity_mode"`
	AttributionTarget string `json:"attribution_target_type"`
	IdentityAssurance string `json:"identity_assurance"`
	IdentitySource    string `json:"identity_source"`
	IdentityVerified  bool   `json:"identity_verified"`
	ClientVerified    bool   `json:"client_verified"`
	FailureReason     string `json:"failure_reason,omitempty"`

	// --- 弱身份个人归因（7.13） ---
	PrincipalID             int    `json:"principal_id"`
	PrincipalCode           string `json:"principal_code"`
	PrincipalName           string `json:"principal_name"`
	CredentialPurposeID     int    `json:"credential_purpose_id"`
	CredentialPurposeCode   string `json:"credential_purpose_code"`
	CredentialPurposeName   string `json:"credential_purpose_name"`
	UsageBusinessDomainID   int    `json:"usage_business_domain_id"`
	UsageBusinessDomainCode string `json:"usage_business_domain_code"`
	UsageBusinessDomainName string `json:"usage_business_domain_name"`
	UsageTeamID             int    `json:"usage_team_id"`
	UsageTeamCode           string `json:"usage_team_code"`
	UsageTeamName           string `json:"usage_team_name"`

	// --- 强身份 Caller（7.13） ---
	CallerID   string `json:"caller_id"`
	CallerName string `json:"caller_name"`

	// --- 应用归因（7.13） ---
	RootAppID                     string `json:"root_app_id"` // 对外使用稳定 app_code
	RootAppName                   string `json:"root_app_name"`
	ApplicationBusinessDomainID   int    `json:"application_business_domain_id"`
	ApplicationBusinessDomainCode string `json:"application_business_domain_code"`
	ApplicationBusinessDomainName string `json:"application_business_domain_name"`
	OwnerTeamID                   int    `json:"owner_team_id"`
	OwnerTeamCode                 string `json:"owner_team_code"`
	OwnerTeamName                 string `json:"owner_team_name"`

	// --- 执行（7.13） ---
	RootRunID          string `json:"root_run_id"`
	CurrentExecutionID string `json:"current_execution_id"`
	ParentExecutionID  string `json:"parent_execution_id"`
	ExecutionType      string `json:"execution_type"`
	ExecutionDepth     int    `json:"execution_depth"`
	WorkflowID         string `json:"workflow_id"`
	AgentID            string `json:"agent_id"`
	TaskID             string `json:"task_id"`
	NodeID             string `json:"node_id"`

	// --- 签名元数据（7.13） ---
	SigningKeyID string `json:"signing_key_id"`
}

// CredentialOnly 报告该可信等级是否仅为“凭证已验证”。
func (t *TrustedAttributionContext) CredentialOnly() bool {
	return t.IdentityAssurance == IdentityAssuranceCredentialOnly
}

// Clone 返回可信归因上下文的深拷贝，避免调用者修改污染复用对象。
func (t *TrustedAttributionContext) Clone() *TrustedAttributionContext {
	if t == nil {
		return nil
	}
	c := *t
	return &c
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

// ---------------------------------------------------------------------------
// 第二批：企业协议 Header / Runtime Mode / 校验限制 / Redis Key
// ---------------------------------------------------------------------------

// 企业身份协议六个 Header（文档 7.2）。只有 DYNAMIC/HYBRID 使用；STATIC 不带。
const (
	AIHeaderContextVersion = "X-AI-Context-Version"
	AIHeaderContext        = "X-AI-Context"
	AIHeaderTimestamp      = "X-AI-Timestamp"
	AIHeaderNonce          = "X-AI-Nonce"
	AIHeaderKeyId          = "X-AI-Key-Id"
	AIHeaderSignature      = "X-AI-Signature"
)

// AIHeaderNames 六个企业身份 Header 名。AIIdentityAuth 一进入就全部删除，
// Channel Header Override（wildcard/regex/显式/{client_header}）一律不得把它们
// 透传给上游 Provider。
var AIHeaderNames = []string{
	AIHeaderContextVersion,
	AIHeaderContext,
	AIHeaderTimestamp,
	AIHeaderNonce,
	AIHeaderKeyId,
	AIHeaderSignature,
}

// IsAIAttributionHeader 判断 header 名是否为六个企业身份 Header 之一（大小写不敏感）。
func IsAIAttributionHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "x-ai-context-version", "x-ai-context", "x-ai-timestamp",
		"x-ai-nonce", "x-ai-key-id", "x-ai-signature":
		return true
	}
	return false
}

// 身份运行时模式（文档 7.8）。AI_ATTRIBUTION_MODE 默认 disabled；非法值 fail-closed。
const (
	AttributionModeDisabled = "disabled"
	AttributionModeAudit    = "audit"
	AttributionModeEnforce  = "enforce"
)

// Runtime 环境变量与校验边界（7.5 / 7.3 / 7.6）。
const (
	AttributionModeEnv          = "AI_ATTRIBUTION_MODE"
	AttributionClockSkewEnv     = "AI_ATTRIBUTION_CLOCK_SKEW_SECONDS"
	AttributionClockSkewMin     = 60
	AttributionClockSkewMax     = 900
	AttributionClockSkewDefault = 300

	AttributionContextMaxEncoded = 8192
	AttributionContextMaxDecoded = 6144
	AttributionNonceMinLen       = 22
	AttributionNonceMaxLen       = 64
	AttributionNonceTTLSeconds   = 600
	AttributionMaxExecutionDepth = 64
	AttributionContextVersion    = "v1"
)

// Context 字段与 Redis Key 固定（7.3 / 7.6 / 6.15.1）。
const (
	ContextFieldRootRunID = "root_run_id"
	ContextFieldRootAppID = "root_app_id"

	IdentityNonceKeyPrefix       = "ai:identity:nonce:"
	CredentialRateLimitKeyPrefix = "ai:credential:rate:"
)

// 身份来源（7.13 identity_source）。
const (
	IdentitySourceToken         = "TOKEN"          // 静态：由 Token/Profile 主数据决定
	IdentitySourceSignedContext = "SIGNED_CONTEXT" // 动态：签名执行上下文
	IdentitySourceHybrid        = "HYBRID"
)

// 第二批固定错误码（文档 7.19 / 冻结验收点 K）。以 AIErrorCode 表达，由中间件
// 转换为 relaykit/types.ErrorCode 下发给 abortWithOpenAiMessage。
type AIErrorCode string

const (
	AIIdentityProfileRequired                   AIErrorCode = "AI_IDENTITY_PROFILE_REQUIRED"
	AIIdentityProfileDisabled                   AIErrorCode = "AI_IDENTITY_PROFILE_DISABLED"
	AIIdentityContextRequired                   AIErrorCode = "AI_IDENTITY_CONTEXT_REQUIRED"
	AIIdentityContextInvalid                    AIErrorCode = "AI_IDENTITY_CONTEXT_INVALID"
	AIIdentityContextTooLarge                   AIErrorCode = "AI_IDENTITY_CONTEXT_TOO_LARGE"
	AIIdentityNonceInvalid                      AIErrorCode = "AI_IDENTITY_NONCE_INVALID"
	AIIdentityTimestampInvalid                  AIErrorCode = "AI_IDENTITY_TIMESTAMP_INVALID"
	AIIdentityKeyInvalid                        AIErrorCode = "AI_IDENTITY_KEY_INVALID"
	AIIdentitySignatureInvalid                  AIErrorCode = "AI_IDENTITY_SIGNATURE_INVALID"
	AIIdentityReplayDetected                    AIErrorCode = "AI_IDENTITY_REPLAY_DETECTED"
	AIIdentityPrincipalDisabled                 AIErrorCode = "AI_IDENTITY_PRINCIPAL_DISABLED"
	AIIdentityPurposeDisabled                   AIErrorCode = "AI_IDENTITY_PURPOSE_DISABLED"
	AIIdentityAppNotAllowed                     AIErrorCode = "AI_IDENTITY_APP_NOT_ALLOWED"
	AIIdentityAppNotBound                       AIErrorCode = "AI_IDENTITY_APP_NOT_BOUND"
	AIIdentityHybridAppMismatch                 AIErrorCode = "AI_IDENTITY_HYBRID_APP_MISMATCH"
	AIIdentityAppDisabled                       AIErrorCode = "AI_IDENTITY_APP_DISABLED"
	AIIdentityTargetInvalid                     AIErrorCode = "AI_IDENTITY_TARGET_INVALID"
	AIIdentityAssuranceInvalid                  AIErrorCode = "AI_IDENTITY_ASSURANCE_INVALID"
	AIIdentityPrincipalRequired                 AIErrorCode = "AI_IDENTITY_PRINCIPAL_REQUIRED"
	AIIdentityPurposeRequired                   AIErrorCode = "AI_IDENTITY_PURPOSE_REQUIRED"
	AIIdentityUsageTeamInvalid                  AIErrorCode = "AI_IDENTITY_USAGE_TEAM_INVALID"
	AIIdentityDuplicateActivePersonalCredential AIErrorCode = "AI_IDENTITY_DUPLICATE_ACTIVE_PERSONAL_CREDENTIAL"
	AIReplayStoreUnavailable                    AIErrorCode = "AI_REPLAY_STORE_UNAVAILABLE"
	AICredentialRateLimitExceeded               AIErrorCode = "AI_CREDENTIAL_RATE_LIMIT_EXCEEDED"
	AICredentialRateLimitStoreUnavailable       AIErrorCode = "AI_CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE"
	AIIdentityAttributionModeInvalid            AIErrorCode = "AI_ATTRIBUTION_MODE_INVALID"
)

// 身份审计 reason_code（写入 model.AIIdentityAuditEvent.ReasonCode，仅安全字段）。
const (
	ReasonCodeReplayStoreUnavailable            = "REPLAY_STORE_UNAVAILABLE"
	ReasonCodeProfileRequired                   = "PROFILE_REQUIRED"
	ReasonCodeProfileDisabled                   = "PROFILE_DISABLED"
	ReasonCodePrincipalDisabled                 = "PRINCIPAL_DISABLED"
	ReasonCodePurposeDisabled                   = "PURPOSE_DISABLED"
	ReasonCodeAppNotAllowed                     = "APP_NOT_ALLOWED"
	ReasonCodeAppNotBound                       = "APP_NOT_BOUND"
	ReasonCodeHybridAppMismatch                 = "HYBRID_APP_MISMATCH"
	ReasonCodeAppDisabled                       = "APP_DISABLED"
	ReasonCodeContextRequired                   = "CONTEXT_REQUIRED"
	ReasonCodeContextInvalid                    = "CONTEXT_INVALID"
	ReasonCodeContextTooLarge                   = "CONTEXT_TOO_LARGE"
	ReasonCodeNonceInvalid                      = "NONCE_INVALID"
	ReasonCodeTimestampInvalid                  = "TIMESTAMP_INVALID"
	ReasonCodeKeyInvalid                        = "KEY_INVALID"
	ReasonCodeSignatureInvalid                  = "SIGNATURE_INVALID"
	ReasonCodeReplayDetected                    = "REPLAY_DETECTED"
	ReasonCodeIdentityModeInvalid               = "IDENTITY_MODE_INVALID"
	ReasonCodeStoreUnavailable                  = "STORE_UNAVAILABLE"
	ReasonCodeAttributionModeInvalid            = "ATTRIBUTION_MODE_INVALID"
	ReasonCodePrincipalRequired                 = "PRINCIPAL_REQUIRED"
	ReasonCodePurposeRequired                   = "PURPOSE_REQUIRED"
	ReasonCodeUsageTeamInvalid                  = "USAGE_TEAM_INVALID"
	ReasonCodeAssuranceInvalid                  = "ASSURANCE_INVALID"
	ReasonCodeTargetInvalid                     = "TARGET_INVALID"
	ReasonCodeDuplicateActivePersonalCredential = "DUPLICATE_ACTIVE_PERSONAL_CREDENTIAL"
)

// AttributionModeValid 校验 AI_ATTRIBUTION_MODE 是否合法。
func AttributionModeValid(mode string) bool {
	switch mode {
	case AttributionModeDisabled, AttributionModeAudit, AttributionModeEnforce:
		return true
	}
	return false
}
