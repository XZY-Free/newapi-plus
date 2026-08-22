package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/tracing"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	newtypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

// attachIdentityAttribution 保存可信归因快照，并将企业身份属性附加到当前 server span（§9.6）。
// OTel 未启用或 Span 未采样时 SetAttributes 为空操作，不改变认证语义。
func attachIdentityAttribution(c *gin.Context, ctx *constant.TrustedAttributionContext) {
	common.SetTrustedAttribution(c, ctx)
	// 继续路径：在此登记 VERIFIED / UNVERIFIED 处置（Final Readiness P1）。
	recordAttributionDecision(c, ctx)
	if ctx == nil {
		return
	}
	if attrs := tracing.EnterpriseAttributes(ctx); len(attrs) > 0 {
		if span := trace.SpanFromContext(c.Request.Context()); span.IsRecording() {
			span.SetAttributes(attrs...)
		}
	}
}

// identityHeaders 是六个企业身份 Header 的原始取值（文档 7.2）。AIIdentityAuth 一进入
// 就复制并全部删除（文档 7.14），保证任何模式、任何跳过路由、任何 c.Next() 之前都
// 不把 X-AI-* 留在请求里，也不被 Channel Header Override 透传给上游。
type identityHeaders struct {
	contextVersion string
	context        string
	timestamp      string
	nonce          string
	keyID          string
	signature      string
}

func extractIdentityHeaders(c *gin.Context) identityHeaders {
	return identityHeaders{
		contextVersion: c.GetHeader(constant.AIHeaderContextVersion),
		context:        c.GetHeader(constant.AIHeaderContext),
		timestamp:      c.GetHeader(constant.AIHeaderTimestamp),
		nonce:          c.GetHeader(constant.AIHeaderNonce),
		keyID:          c.GetHeader(constant.AIHeaderKeyId),
		signature:      c.GetHeader(constant.AIHeaderSignature),
	}
}

// deleteIdentityHeaders 删除六个企业身份 Header（大小写不敏感由 net/http 处理）。
func deleteIdentityHeaders(c *gin.Context) {
	for _, h := range constant.AIHeaderNames {
		c.Request.Header.Del(h)
	}
}

// identityDecision 是 AIIdentityAuth 在治理决策点得出的最终身份验证处置（三态：
// VERIFIED / UNVERIFIED / REJECTED），携带 identity_mode / identity_assurance /
// reason_code 供指标标签。存于 ContextKeyIdentityDecision，由 defer 读取发出指标。
type identityDecision struct {
	result     string
	mode       string
	assurance  string
	reasonCode string
}

func setIdentityDecision(c *gin.Context, d *identityDecision) {
	c.Set(string(constant.ContextKeyIdentityDecision), d)
}

// markIdentityVerified 在身份建立（继续）时登记 VERIFIED。
func markIdentityVerified(c *gin.Context, ctx *constant.TrustedAttributionContext) {
	if ctx == nil {
		return
	}
	setIdentityDecision(c, &identityDecision{
		result:    constant.IdentityAuditResultVerified,
		mode:      ctx.IdentityMode,
		assurance: ctx.IdentityAssurance,
	})
}

// markIdentityUnverified 在 audit 降级放行（继续）时登记 UNVERIFIED，携带降级原因。
// audit 失败+继续必须是 UNVERIFIED，绝不 REJECTED。
func markIdentityUnverified(c *gin.Context, ctx *constant.TrustedAttributionContext) {
	if ctx == nil {
		return
	}
	setIdentityDecision(c, &identityDecision{
		result:     constant.IdentityAuditResultUnverified,
		mode:       ctx.IdentityMode,
		assurance:  ctx.IdentityAssurance,
		reasonCode: ctx.FailureReason,
	})
}

// markIdentityRejected 在 ENFORCE 真正拦截（或非法模式 503）时登记 REJECTED，
// reason_code 保留（SIGNATURE_INVALID / APP_NOT_BOUND / PROFILE_DISABLED /
// REPLAY_DETECTED / ATTRIBUTION_MODE_INVALID 等）。
func markIdentityRejected(c *gin.Context, mode, assurance, reasonCode string) {
	setIdentityDecision(c, &identityDecision{
		result:     constant.IdentityAuditResultRejected,
		mode:       mode,
		assurance:  assurance,
		reasonCode: reasonCode,
	})
}

// verificationResultForContext 依据归因上下文得出继续路径的三态处置
// （Final Readiness P1 语义）：IdentityVerified=true → VERIFIED；否则 → UNVERIFIED。
// 注意：audit 失败+继续必须一律 UNVERIFIED，绝不因 FailureReason 存在而误判 REJECTED。
// nil 上下文（未做治理判定）返回空串。
func verificationResultForContext(ctx *constant.TrustedAttributionContext) string {
	if ctx == nil {
		return ""
	}
	if ctx.IdentityVerified {
		return constant.IdentityAuditResultVerified
	}
	return constant.IdentityAuditResultUnverified
}

// recordAttributionDecision 在继续路径（attachIdentityAttribution 已附着归因）登记
// 三态处置：IdentityVerified=true → VERIFIED；否则（audit 降级）→ UNVERIFIED。
func recordAttributionDecision(c *gin.Context, ctx *constant.TrustedAttributionContext) {
	switch verificationResultForContext(ctx) {
	case constant.IdentityAuditResultVerified:
		markIdentityVerified(c, ctx)
	case constant.IdentityAuditResultUnverified:
		markIdentityUnverified(c, ctx)
	}
}

// recordIdentityVerificationDecision 在 defer 中读取处置并发出恰一次指标。
// 未登记处置（disabled 模式 / 非消费查询 / 未做治理判定）时不计数。
func recordIdentityVerificationDecision(c *gin.Context) {
	v, ok := c.Get(string(constant.ContextKeyIdentityDecision))
	if !ok {
		return
	}
	d, ok := v.(*identityDecision)
	if !ok || d == nil {
		return
	}
	tracing.RecordIdentityVerification(d.mode, d.assurance, d.reasonCode, d.result)
}

// AIIdentityAuth 运行时身份认证中间件（文档 7）。顺序 TokenAuth → AIIdentityAuth →
// AICredentialRateLimit → 原 ModelRequestRateLimit → Distribute。
//
// disabled：仅删除入站 X-AI-* 并继续，不做归因（文档 7.8）。
// audit：合法请求生成完整可信上下文；非法/缺失/强身份验证失败（含重放）降级放行并审计；
// enforce：只有符合组合规则的消费请求才能继续。
// 非法 AI_ATTRIBUTION_MODE：不回退 disabled；消费请求清头后 503 并审计
// ATTRIBUTION_MODE_INVALID，非消费查询清头继续（7.8 / 7.20）。
func AIIdentityAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// Final Readiness P1：在治理决策点登记最终身份验证处置，每个请求恰一次。
		// defer 在函数返回（含 abort/c.Next() 完成后）时读取处置并发出指标。
		defer recordIdentityVerificationDecision(c)

		hdr := extractIdentityHeaders(c)
		deleteIdentityHeaders(c)

		mode := service.GetAttributionMode()
		if !constant.AttributionModeValid(mode) {
			// 非法 AI_ATTRIBUTION_MODE：不回退 disabled。消费请求 fail-closed 503。
			if service.IsAttributionRequired(c.Request.Method, c.Request.URL.Path) {
				writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
					RequestId:   c.GetString(common.RequestIdKey),
					Result:      constant.IdentityAuditResultRejected,
					ReasonCode:  constant.ReasonCodeAttributionModeInvalid,
					HttpMethod:  c.Request.Method,
					RequestPath: c.Request.URL.Path,
					ClientIp:    c.ClientIP(),
				})
				markIdentityRejected(c, mode, "", constant.ReasonCodeAttributionModeInvalid)
				abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "身份归因模式配置非法", relaytypes.ErrorCode(constant.AIIdentityAttributionModeInvalid))
				return
			}
			c.Next()
			return
		}
		if mode == constant.AttributionModeDisabled {
			c.Next()
			return
		}

		// 只读/查询/下载/fetch 等非“新消费”端点不要求身份验证（7.16），仍已清头。
		if !service.IsAttributionRequired(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		tokenID := c.GetInt("token_id")
		if tokenID <= 0 {
			// 消费入口上缺少 token：audit 放行（token-only 降级），enforce 拒绝。
			writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
				RequestId:   c.GetString(common.RequestIdKey),
				Result:      identityAuditResultForMode(mode),
				ReasonCode:  constant.ReasonCodeProfileRequired,
				HttpMethod:  c.Request.Method,
				RequestPath: c.Request.URL.Path,
				ClientIp:    c.ClientIP(),
			})
			if mode == constant.AttributionModeEnforce {
				markIdentityRejected(c, "", "", constant.ReasonCodeProfileRequired)
				abortWithOpenAiMessage(c, http.StatusUnauthorized, "缺少有效的 API Key", relaytypes.ErrorCode(constant.AIIdentityProfileRequired))
				return
			}
			attachIdentityAttribution(c, degradedTokenContext(0))
			c.Next()
			return
		}

		snapshot, err := service.GetIdentitySnapshotByTokenID(tokenID)
		if err != nil {
			common.SysError("AIIdentityAuth snapshot error: " + err.Error())
			writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
				RequestId:   c.GetString(common.RequestIdKey),
				TokenId:     tokenID,
				Result:      identityAuditResultForMode(mode),
				ReasonCode:  constant.ReasonCodeStoreUnavailable,
				HttpMethod:  c.Request.Method,
				RequestPath: c.Request.URL.Path,
				ClientIp:    c.ClientIP(),
			})
			if mode == constant.AttributionModeEnforce {
				markIdentityRejected(c, "", "", constant.ReasonCodeStoreUnavailable)
				abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "身份快照不可用", relaytypes.ErrorCode(constant.AIIdentityProfileRequired))
				return
			}
			attachIdentityAttribution(c, degradedTokenContext(tokenID))
			c.Next()
			return
		}
		if snapshot == nil {
			handleProfileMissing(c, mode, tokenID)
			return
		}
		if !snapshot.Enabled {
			handleProfileDisabled(c, mode, snapshot)
			return
		}
		if failure := snapshotValidityFailure(snapshot); failure != nil {
			handleSnapshotInvalid(c, mode, snapshot, failure)
			return
		}

		switch snapshot.IdentityMode {
		case constant.IdentityModeStatic:
			handleStaticAttribution(c, mode, snapshot)
		case constant.IdentityModeDynamic, constant.IdentityModeHybrid:
			handleSignedAttribution(c, mode, hdr, snapshot)
		default:
			handleIdentityModeInvalid(c, mode, snapshot)
		}
	}
}

// degradedTokenContext 构造仅含 token 静态事实的降级上下文（7.13 / 7.20）。
func degradedTokenContext(tokenID int) *constant.TrustedAttributionContext {
	// 7.8 AUDIT 冻结：TokenAuth 已成功（token_id>0）说明 API Key 已验证，凭证事实必须保留；
	// 缺失 Profile 只降级 identity/client 验证，不得抹掉 credential_verified。token_id<=0 仍视为未验证。
	return &constant.TrustedAttributionContext{
		TokenID:            tokenID,
		CredentialVerified: tokenID > 0,
		IdentitySource:     constant.IdentitySourceToken,
		IdentityVerified:   false,
		ClientVerified:     false,
	}
}

// degradedProfileContext 构造仅含 token+profile 静态事实的降级上下文。强身份验证失败
// 或存储不可用时在 audit 模式采纳，绝不采纳客户端 app/run/execution/signing_key_id。
func degradedProfileContext(snapshot *newtypes.IdentitySnapshot, reason string) *constant.TrustedAttributionContext {
	return &constant.TrustedAttributionContext{
		TokenID:            snapshot.TokenID,
		ProfileID:          snapshot.ProfileID,
		CredentialVerified: true,
		Environment:        snapshot.Environment,
		IdentityMode:       snapshot.IdentityMode,
		AttributionTarget:  snapshot.AttributionTarget,
		IdentityAssurance:  snapshot.IdentityAssurance,
		IdentitySource:     constant.IdentitySourceToken,
		IdentityVerified:   false,
		ClientVerified:     false,
		FailureReason:      reason,
		CallerID:           snapshot.CallerID,
		CallerName:         snapshot.CallerName,
	}
}

func handleProfileMissing(c *gin.Context, mode string, tokenID int) {
	writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
		RequestId:   c.GetString(common.RequestIdKey),
		TokenId:     tokenID,
		Result:      identityAuditResultForMode(mode),
		ReasonCode:  constant.ReasonCodeProfileRequired,
		HttpMethod:  c.Request.Method,
		RequestPath: c.Request.URL.Path,
		ClientIp:    c.ClientIP(),
	})
	if mode == constant.AttributionModeEnforce {
		markIdentityRejected(c, "", "", constant.ReasonCodeProfileRequired)
		abortWithOpenAiMessage(c, http.StatusForbidden, "Token 未登记企业身份 Profile", relaytypes.ErrorCode(constant.AIIdentityProfileRequired))
		return
	}
	attachIdentityAttribution(c, degradedTokenContext(tokenID))
	c.Next()
}

func handleProfileDisabled(c *gin.Context, mode string, snapshot *newtypes.IdentitySnapshot) {
	writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
		RequestId:   c.GetString(common.RequestIdKey),
		TokenId:     snapshot.TokenID,
		ProfileId:   snapshot.ProfileID,
		Result:      identityAuditResultForMode(mode),
		ReasonCode:  constant.ReasonCodeProfileDisabled,
		HttpMethod:  c.Request.Method,
		RequestPath: c.Request.URL.Path,
		ClientIp:    c.ClientIP(),
	})
	if mode == constant.AttributionModeEnforce {
		markIdentityRejected(c, snapshot.IdentityMode, snapshot.IdentityAssurance, constant.ReasonCodeProfileDisabled)
		abortWithOpenAiMessage(c, http.StatusForbidden, "Identity Profile 已停用", relaytypes.ErrorCode(constant.AIIdentityProfileDisabled))
		return
	}
	attachIdentityAttribution(c, degradedProfileContext(snapshot, constant.ReasonCodeProfileDisabled))
	c.Next()
}

func handleIdentityModeInvalid(c *gin.Context, mode string, snapshot *newtypes.IdentitySnapshot) {
	writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
		RequestId:   c.GetString(common.RequestIdKey),
		TokenId:     snapshot.TokenID,
		ProfileId:   snapshot.ProfileID,
		Result:      identityAuditResultForMode(mode),
		ReasonCode:  constant.ReasonCodeIdentityModeInvalid,
		HttpMethod:  c.Request.Method,
		RequestPath: c.Request.URL.Path,
		ClientIp:    c.ClientIP(),
	})
	if mode == constant.AttributionModeEnforce {
		markIdentityRejected(c, snapshot.IdentityMode, snapshot.IdentityAssurance, constant.ReasonCodeIdentityModeInvalid)
		abortWithOpenAiMessage(c, http.StatusForbidden, "Identity Profile 身份模式非法", relaytypes.ErrorCode(constant.AIIdentityProfileDisabled))
		return
	}
	attachIdentityAttribution(c, degradedProfileContext(snapshot, constant.ReasonCodeIdentityModeInvalid))
	c.Next()
}

// aiAvailabilityFailure 描述快照结构无效或 STATIC 归因中被引用实体不可用。
type aiAvailabilityFailure struct {
	code    constant.AIErrorCode
	reason  string
	message string
}

// snapshotValidityFailure 检测快照结构是否可归因（7.19 固定 reason_code）：
// target/assurance 非法、STATIC/PRINCIPAL 缺 principal/purpose/usage team、
// STATIC/APPLICATION 无绑定应用。运行时 fail-closed。
func snapshotValidityFailure(snapshot *newtypes.IdentitySnapshot) *aiAvailabilityFailure {
	if !constant.AttributionTargetValid(snapshot.AttributionTarget) {
		return &aiAvailabilityFailure{constant.AIIdentityTargetInvalid, constant.ReasonCodeTargetInvalid, "Identity Profile 归因目标非法"}
	}
	if !constant.IdentityAssuranceValid(snapshot.IdentityAssurance) {
		return &aiAvailabilityFailure{constant.AIIdentityAssuranceInvalid, constant.ReasonCodeAssuranceInvalid, "Identity Profile 身份可信等级非法"}
	}
	if snapshot.AttributionTarget == constant.AttributionTargetPrincipal {
		if snapshot.PrincipalID <= 0 || snapshot.PrincipalCode == "" {
			return &aiAvailabilityFailure{constant.AIIdentityPrincipalRequired, constant.ReasonCodePrincipalRequired, "Identity Profile 缺少使用主体"}
		}
		if snapshot.CredentialPurposeID <= 0 {
			return &aiAvailabilityFailure{constant.AIIdentityPurposeRequired, constant.ReasonCodePurposeRequired, "Identity Profile 缺少凭证用途"}
		}
		if snapshot.UsageTeamID <= 0 {
			return &aiAvailabilityFailure{constant.AIIdentityUsageTeamInvalid, constant.ReasonCodeUsageTeamInvalid, "Identity Profile 缺少使用团队"}
		}
	}
	if snapshot.AttributionTarget == constant.AttributionTargetApplication && len(snapshot.Applications) == 0 {
		return &aiAvailabilityFailure{constant.AIIdentityAppNotBound, constant.ReasonCodeAppNotBound, "Identity Profile 未绑定固定应用"}
	}
	return nil
}

// handleSnapshotInvalid 处理结构性无效的快照：enforce 拒绝；audit 降级（仅保留
// token/profile 静态事实）。
func handleSnapshotInvalid(c *gin.Context, mode string, snapshot *newtypes.IdentitySnapshot, failure *aiAvailabilityFailure) {
	writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
		RequestId:   c.GetString(common.RequestIdKey),
		TokenId:     snapshot.TokenID,
		ProfileId:   snapshot.ProfileID,
		Result:      identityAuditResultForMode(mode),
		ReasonCode:  failure.reason,
		HttpMethod:  c.Request.Method,
		RequestPath: c.Request.URL.Path,
		ClientIp:    c.ClientIP(),
	})
	if mode == constant.AttributionModeEnforce {
		markIdentityRejected(c, snapshot.IdentityMode, snapshot.IdentityAssurance, failure.reason)
		abortWithOpenAiMessage(c, http.StatusForbidden, failure.message, relaytypes.ErrorCode(failure.code))
		return
	}
	attachIdentityAttribution(c, degradedProfileContext(snapshot, failure.reason))
	c.Next()
}

// staticAvailabilityFailure 依据快照检测 STATIC 归因的可用性（文档 7.9 / 7.10 / 验收 E）。
func staticAvailabilityFailure(snapshot *newtypes.IdentitySnapshot) *aiAvailabilityFailure {
	switch snapshot.AttributionTarget {
	case constant.AttributionTargetPrincipal:
		if !snapshot.PrincipalEnabled {
			return &aiAvailabilityFailure{constant.AIIdentityPrincipalDisabled, constant.ReasonCodePrincipalDisabled, "使用主体已停用"}
		}
		if !snapshot.CredentialPurposeEnabled {
			return &aiAvailabilityFailure{constant.AIIdentityPurposeDisabled, constant.ReasonCodePurposeDisabled, "凭证用途已停用"}
		}
	case constant.AttributionTargetApplication:
		for _, a := range snapshot.Applications {
			if !a.AppEnabled {
				return &aiAvailabilityFailure{constant.AIIdentityAppDisabled, constant.ReasonCodeAppDisabled, "固定应用已停用"}
			}
		}
	}
	return nil
}

func handleStaticAttribution(c *gin.Context, mode string, snapshot *newtypes.IdentitySnapshot) {
	ctx := buildStaticTrustedContext(snapshot)
	if failure := staticAvailabilityFailure(snapshot); failure != nil {
		writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
			RequestId:           c.GetString(common.RequestIdKey),
			TokenId:             snapshot.TokenID,
			ProfileId:           snapshot.ProfileID,
			PrincipalId:         snapshot.PrincipalID,
			CredentialPurposeId: snapshot.CredentialPurposeID,
			IdentityMode:        snapshot.IdentityMode,
			IdentityAssurance:   snapshot.IdentityAssurance,
			Result:              identityAuditResultForMode(mode),
			ReasonCode:          failure.reason,
			HttpMethod:          c.Request.Method,
			RequestPath:         c.Request.URL.Path,
			ClientIp:            c.ClientIP(),
		})
		if mode == constant.AttributionModeEnforce {
			markIdentityRejected(c, snapshot.IdentityMode, snapshot.IdentityAssurance, failure.reason)
			abortWithOpenAiMessage(c, http.StatusForbidden, failure.message, relaytypes.ErrorCode(failure.code))
			return
		}
		// audit 降级：清除被停用实体相关的归因，identity/client 均 false。
		clearDisabledAttribution(ctx, failure.reason)
	}
	attachIdentityAttribution(c, ctx)
	c.Next()
}

// clearDisabledAttribution 在 STATIC audit 降级时清除被停用实体相关的归因字段，
// 并置 identity_verified/client_verified=false、failure_reason。
func clearDisabledAttribution(ctx *constant.TrustedAttributionContext, reason string) {
	switch reason {
	case constant.ReasonCodePrincipalDisabled:
		ctx.PrincipalID = 0
		ctx.PrincipalCode = ""
		ctx.PrincipalName = ""
		ctx.UsageBusinessDomainID = 0
		ctx.UsageBusinessDomainCode = ""
		ctx.UsageBusinessDomainName = ""
		ctx.UsageTeamID = 0
		ctx.UsageTeamCode = ""
		ctx.UsageTeamName = ""
	case constant.ReasonCodePurposeDisabled:
		ctx.CredentialPurposeID = 0
		ctx.CredentialPurposeCode = ""
		ctx.CredentialPurposeName = ""
	case constant.ReasonCodeAppDisabled:
		ctx.RootAppID = ""
		ctx.RootAppName = ""
		ctx.ApplicationBusinessDomainID = 0
		ctx.ApplicationBusinessDomainCode = ""
		ctx.ApplicationBusinessDomainName = ""
		ctx.OwnerTeamID = 0
		ctx.OwnerTeamCode = ""
		ctx.OwnerTeamName = ""
	}
	ctx.IdentityVerified = false
	ctx.ClientVerified = false
	ctx.FailureReason = reason
}

// buildStaticTrustedContext 构造 STATIC 可信上下文（文档 7.9 / 7.10）。
// credential_verified=true、client_verified=false；caller 字段为空；
// PRINCIPAL 填充 principal/usage domain/team/purpose；APPLICATION 填充固定 root_app。
func buildStaticTrustedContext(snapshot *newtypes.IdentitySnapshot) *constant.TrustedAttributionContext {
	ctx := &constant.TrustedAttributionContext{
		TokenID:                 snapshot.TokenID,
		ProfileID:               snapshot.ProfileID,
		CredentialVerified:      true,
		Environment:             snapshot.Environment,
		IdentityMode:            snapshot.IdentityMode,
		AttributionTarget:       snapshot.AttributionTarget,
		IdentityAssurance:       snapshot.IdentityAssurance,
		IdentitySource:          constant.IdentitySourceToken,
		IdentityVerified:        true,
		ClientVerified:          false,
		PrincipalID:             snapshot.PrincipalID,
		PrincipalCode:           snapshot.PrincipalCode,
		PrincipalName:           snapshot.PrincipalName,
		CredentialPurposeID:     snapshot.CredentialPurposeID,
		CredentialPurposeCode:   snapshot.CredentialPurposeCode,
		CredentialPurposeName:   snapshot.CredentialPurposeName,
		UsageBusinessDomainID:   snapshot.UsageDomainID,
		UsageBusinessDomainCode: snapshot.UsageDomainCode,
		UsageBusinessDomainName: snapshot.UsageDomainName,
		UsageTeamID:             snapshot.UsageTeamID,
		UsageTeamCode:           snapshot.UsageTeamCode,
		UsageTeamName:           snapshot.UsageTeamName,
	}
	if snapshot.AttributionTarget == constant.AttributionTargetApplication && len(snapshot.Applications) > 0 {
		a := snapshot.Applications[0]
		ctx.RootAppID = a.AppCode
		ctx.RootAppName = a.AppName
		ctx.ApplicationBusinessDomainID = a.BusinessDomainID
		ctx.ApplicationBusinessDomainCode = a.BusinessDomainCode
		ctx.ApplicationBusinessDomainName = a.BusinessDomainName
		ctx.OwnerTeamID = a.OwnerTeamID
		ctx.OwnerTeamCode = a.OwnerTeamCode
		ctx.OwnerTeamName = a.OwnerTeamName
	}
	return ctx
}

// aiIdentityFailure 是 middleware 层对强身份验证失败的统一描述，映射到 HTTP 状态与错误码。
type aiIdentityFailure struct {
	status     int
	code       constant.AIErrorCode
	reason     string
	message    string
	claimedApp string
}

func handleSignedAttribution(c *gin.Context, mode string, hdr identityHeaders, snapshot *newtypes.IdentitySnapshot) {
	ctx, failure, storeErr := verifySignedRequest(c, hdr, snapshot)
	if storeErr != nil {
		handleIdentityRedisDown(c, mode, snapshot)
		return
	}
	if failure != nil {
		// enforce 拒绝全部强身份失败（含重放）；audit 一律降级放行（7.8/7.20），
		// 重放同样走降级：保留凭证已验证静态事实、不采纳客户端归因、写 UNVERIFIED REPLAY_DETECTED。
		if mode == constant.AttributionModeEnforce {
			writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
				RequestId:           c.GetString(common.RequestIdKey),
				TokenId:             snapshot.TokenID,
				ProfileId:           snapshot.ProfileID,
				CallerId:            snapshot.CallerID,
				PrincipalId:         snapshot.PrincipalID,
				CredentialPurposeId: snapshot.CredentialPurposeID,
				IdentityMode:        snapshot.IdentityMode,
				IdentityAssurance:   snapshot.IdentityAssurance,
				Result:              constant.IdentityAuditResultRejected,
				ReasonCode:          failure.reason,
				ClaimedRootAppId:    failure.claimedApp,
				HttpMethod:          c.Request.Method,
				RequestPath:         c.Request.URL.Path,
				ClientIp:            c.ClientIP(),
			})
			markIdentityRejected(c, snapshot.IdentityMode, snapshot.IdentityAssurance, failure.reason)
			abortWithOpenAiMessage(c, failure.status, failure.message, relaytypes.ErrorCode(failure.code))
			return
		}
		// audit 降级：UNVERIFIED，仅保留 token/profile 静态事实，不采纳客户端
		// app/run/execution/signing_key_id。仅当服务验证在解码/HMAC/时间戳/密钥之后
		// 认定 root_app_id 为语法合法的未绑定声明时，把该声明写入审计 claimed_root_app_id
		// （6.11），绝不进入正式归因（7.8）。
		writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
			RequestId:           c.GetString(common.RequestIdKey),
			TokenId:             snapshot.TokenID,
			ProfileId:           snapshot.ProfileID,
			CallerId:            snapshot.CallerID,
			PrincipalId:         snapshot.PrincipalID,
			CredentialPurposeId: snapshot.CredentialPurposeID,
			IdentityMode:        snapshot.IdentityMode,
			IdentityAssurance:   snapshot.IdentityAssurance,
			Result:              constant.IdentityAuditResultUnverified,
			ReasonCode:          failure.reason,
			ClaimedRootAppId:    failure.claimedApp,
			HttpMethod:          c.Request.Method,
			RequestPath:         c.Request.URL.Path,
			ClientIp:            c.ClientIP(),
		})
		attachIdentityAttribution(c, degradedProfileContext(snapshot, failure.reason))
		c.Next()
		return
	}
	attachIdentityAttribution(c, ctx)
	c.Next()
}

// verifySignedRequest 委托 service.VerifyStrongIdentity 按文档 7.6 固定顺序验证
// DYNAMIC/HYBRID 强身份：格式 → Profile/Key → Timestamp → HMAC → Caller/App 配置 →
// Redis SET NX → Context 语义。
//
// 返回 storeErr 表示防重放存储（Redis）不可用，交由 mode 分支处理；返回 failure 表示
// 确定性验证失败（enforce 拒绝、audit 降级放行，含重放）。
func verifySignedRequest(c *gin.Context, hdr identityHeaders, snapshot *newtypes.IdentitySnapshot) (*constant.TrustedAttributionContext, *aiIdentityFailure, error) {
	svcHdr := &service.AIIdentityHeaders{
		ContextVersion: hdr.contextVersion,
		Context:        hdr.context,
		Timestamp:      hdr.timestamp,
		Nonce:          hdr.nonce,
		KeyId:          hdr.keyID,
		Signature:      hdr.signature,
	}
	method, path := originalMethodPath(c)
	ctx, failure := service.VerifyStrongIdentity(snapshot, svcHdr, method, path, common.GetTimestamp())
	if failure == nil {
		return ctx, nil, nil
	}
	if failure.StoreUnavailable {
		return nil, nil, service.ErrNonceStoreUnavailable
	}
	return nil, &aiIdentityFailure{
		status:     aiFailureStatus(constant.AIErrorCode(failure.Code)),
		code:       constant.AIErrorCode(failure.Code),
		reason:     failure.Reason,
		message:    aiFailureMessage(constant.AIErrorCode(failure.Code)),
		claimedApp: failure.ClaimedRootAppID,
	}, nil
}

// aiFailureStatus 映射固定错误码到 HTTP 状态。
func aiFailureStatus(code constant.AIErrorCode) int {
	switch code {
	case constant.AIIdentityContextRequired, constant.AIIdentityContextInvalid,
		constant.AIIdentityContextTooLarge, constant.AIIdentityNonceInvalid,
		constant.AIIdentityTimestampInvalid, constant.AIIdentityKeyInvalid:
		return http.StatusBadRequest
	default:
		return http.StatusForbidden
	}
}

// aiFailureMessage 返回固定错误码对应的用户可见消息。
func aiFailureMessage(code constant.AIErrorCode) string {
	switch code {
	case constant.AIIdentityContextRequired:
		return "缺少 X-AI-Context"
	case constant.AIIdentityContextInvalid:
		return "X-AI-Context 无效"
	case constant.AIIdentityContextTooLarge:
		return "X-AI-Context 超出长度上限"
	case constant.AIIdentityNonceInvalid:
		return "X-AI-Nonce 无效"
	case constant.AIIdentityTimestampInvalid:
		return "X-AI-Timestamp 无效或超出时钟偏差"
	case constant.AIIdentityKeyInvalid:
		return "签名密钥不可用"
	case constant.AIIdentitySignatureInvalid:
		return "签名验证失败"
	case constant.AIIdentityReplayDetected:
		return "检测到重放请求"
	case constant.AIIdentityAppNotAllowed:
		return "应用未绑定或不匹配"
	case constant.AIIdentityAppNotBound:
		return "应用未绑定"
	case constant.AIIdentityHybridAppMismatch:
		return "应用与固定应用不匹配"
	case constant.AIIdentityAppDisabled:
		return "应用已停用"
	default:
		return "企业身份验证失败"
	}
}

// handleIdentityRedisDown 处理防重放存储不可用（文档 7.7 / 验收 D）：
// enforce 503 AI_REPLAY_STORE_UNAVAILABLE；audit 降级未验证，不采纳客户端 app/run/execution。
func handleIdentityRedisDown(c *gin.Context, mode string, snapshot *newtypes.IdentitySnapshot) {
	writeIdentityAudit(c, mode, &model.AIIdentityAuditEvent{
		RequestId:         c.GetString(common.RequestIdKey),
		TokenId:           snapshot.TokenID,
		ProfileId:         snapshot.ProfileID,
		CallerId:          snapshot.CallerID,
		IdentityMode:      snapshot.IdentityMode,
		IdentityAssurance: snapshot.IdentityAssurance,
		Result:            identityAuditResultForMode(mode),
		ReasonCode:        constant.ReasonCodeReplayStoreUnavailable,
		HttpMethod:        c.Request.Method,
		RequestPath:       c.Request.URL.Path,
		ClientIp:          c.ClientIP(),
	})
	if mode == constant.AttributionModeEnforce {
		markIdentityRejected(c, snapshot.IdentityMode, snapshot.IdentityAssurance, constant.ReasonCodeReplayStoreUnavailable)
		abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "重放防护存储不可用", relaytypes.ErrorCode(constant.AIReplayStoreUnavailable))
		return
	}
	// audit 降级：仅保留由 Token/Profile 静态确定的事实，identity_verified=false。
	attachIdentityAttribution(c, degradedProfileContext(snapshot, constant.ReasonCodeReplayStoreUnavailable))
	c.Next()
}

// originalMethodPath 返回 canonical 签名所需的“外部入站 method/path”。
// 视频类 converter（Kling/Jimeng）在改写 URL 前已把原始 method/path 存入 context（7.18/J），
// 因此签名验证使用入站值，而非被 converter 改写后的值。
func originalMethodPath(c *gin.Context) (string, string) {
	method := c.Request.Method
	if v, ok := c.Get(string(constant.ContextKeyAIOriginalMethod)); ok {
		if s, ok := v.(string); ok && s != "" {
			method = s
		}
	}
	path := c.Request.URL.Path
	if v, ok := c.Get(string(constant.ContextKeyAIOriginalPath)); ok {
		if s, ok := v.(string); ok && s != "" {
			path = s
		}
	}
	return method, path
}

// identityAuditResultForMode 依据实际处置决定 result，是 result 的唯一判定，避免每个
// callsite 重复 mode 判断：enforce 命中即 abort → REJECTED；audit 降级放行 → UNVERIFIED。
// disabled 不落库（由 writeIdentityAudit 过滤），故这里只需区分 enforce 与其余。
func identityAuditResultForMode(mode string) string {
	if mode == constant.AttributionModeEnforce {
		return constant.IdentityAuditResultRejected
	}
	return constant.IdentityAuditResultUnverified
}

// writeIdentityAudit 仅在 audit/enforce 模式下落审计事件（disabled 不写）。
func writeIdentityAudit(c *gin.Context, mode string, ev *model.AIIdentityAuditEvent) {
	if mode == constant.AttributionModeDisabled {
		return
	}
	if err := service.WriteIdentityAuditEvent(ev); err != nil {
		common.SysError("write identity audit event error: " + err.Error())
	}
}
