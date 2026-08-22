package middleware

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// AICredentialRateLimit 企业 Profile 级凭证请求限流（文档 6.15.1 / 7.17）。
//
// 只在以下条件同时满足时执行：
//   - Runtime Mode != disabled；
//   - Profile 已成功解析（可信上下文已写入）；
//   - Profile rate_limit_enabled=true。
//
// 限流 Key 使用 profile_id（同一人不同 Purpose 拥有独立桶），绝不用 user_id/principal_id。
// 顺序：TokenAuth → AIIdentityAuth → AICredentialRateLimit → 原 ModelRequestRateLimit → Distribute。
//
// 非法 AI_ATTRIBUTION_MODE 独立 fail-closed：503 AI_ATTRIBUTION_MODE_INVALID。
// Redis 故障：audit 放行并审计；enforce 且该 Profile 启用了限流时 503（fail closed）。
// 超过阈值：429 AI_CREDENTIAL_RATE_LIMIT_EXCEEDED，Provider 不收到请求，不修改原限流语义。
//
// member 每请求唯一：优先取 request_id；缺失时生成唯一成员，避免 ZADD 同成员被覆盖
// 导致计数失真。
func AICredentialRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		mode := service.GetAttributionMode()
		if !constant.AttributionModeValid(mode) {
			// 非法模式独立 503，不进终态。
			abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "身份归因模式配置非法", relaytypes.ErrorCode(constant.AIIdentityAttributionModeInvalid))
			return
		}
		if mode == constant.AttributionModeDisabled {
			c.Next()
			return
		}
		ctx, ok := common.GetTrustedAttribution(c)
		if !ok || ctx == nil || ctx.ProfileID <= 0 {
			c.Next()
			return
		}
		snapshot, err := service.GetIdentitySnapshotByTokenID(ctx.TokenID)
		if err != nil || snapshot == nil {
			c.Next()
			return
		}
		rl := snapshot.RateLimit
		if !rl.Enabled || rl.WindowSeconds <= 0 || rl.MaxRequests <= 0 {
			c.Next()
			return
		}

		member := uniqueRateLimitMember(c.GetString(common.RequestIdKey))
		allowed, err := service.AllowProfileRateLimit(c.Request.Context(), ctx.ProfileID, rl.WindowSeconds, rl.MaxRequests, member)
		if err != nil {
			writeCredentialRateLimitAudit(c, mode, ctx, constant.ReasonCodeCredentialRateLimitStoreUnavailable)
			if mode == constant.AttributionModeEnforce {
				abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "凭证限流存储不可用", relaytypes.ErrorCode(constant.AICredentialRateLimitStoreUnavailable))
				return
			}
			c.Next()
			return
		}
		if !allowed {
			writeCredentialRateLimitAudit(c, mode, ctx, constant.ReasonCodeCredentialRateLimitExceeded)
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试", relaytypes.ErrorCode(constant.AICredentialRateLimitExceeded))
			return
		}
		c.Next()
	}
}

// rateLimitMemberSeq 为请求限流成员分配进程内唯一单调序号，防止同一毫秒内同 request_id
// 产生成员碰撞（ZADD 同 member 覆盖导致计数失真）。
var rateLimitMemberSeq atomic.Int64

// uniqueRateLimitMember 生成每请求唯一的限流成员，形状为 <request_id>:<毫秒时间戳>:<单调序号>。
// 每个进入限流的请求都必须拥有独立 member：即便 request_id 重复、即便同毫秒并发，
// 末尾单调序号也保证不重复，避免 ZADD 以同 member 覆盖而漏计。
func uniqueRateLimitMember(requestID string) string {
	if requestID == "" {
		requestID = "req"
	}
	return requestID + ":" + strconv.FormatInt(time.Now().UnixMilli(), 10) + ":" + strconv.FormatInt(rateLimitMemberSeq.Add(1), 10)
}

// writeCredentialRateLimitAudit 落凭证限流审计事件（仅安全字段，禁止 API Key/正文）。
func writeCredentialRateLimitAudit(c *gin.Context, mode string, ctx *constant.TrustedAttributionContext, reason string) {
	if mode == constant.AttributionModeDisabled {
		return
	}
	// 处置语义：CREDENTIAL_RATE_LIMIT_EXCEEDED 两个模式都实际 429 阻断 → REJECTED；
	// CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE 只有 enforce 503 fail-closed → REJECTED，
	// audit 放行 → UNVERIFIED。
	result := constant.IdentityAuditResultUnverified
	if reason == constant.ReasonCodeCredentialRateLimitExceeded || mode == constant.AttributionModeEnforce {
		result = constant.IdentityAuditResultRejected
	}
	ev := &model.AIIdentityAuditEvent{
		RequestId:           c.GetString(common.RequestIdKey),
		TokenId:             ctx.TokenID,
		ProfileId:           ctx.ProfileID,
		CallerId:            ctx.CallerID,
		PrincipalId:         ctx.PrincipalID,
		CredentialPurposeId: ctx.CredentialPurposeID,
		IdentityMode:        ctx.IdentityMode,
		IdentityAssurance:   ctx.IdentityAssurance,
		Result:              result,
		ReasonCode:          reason,
		HttpMethod:          c.Request.Method,
		RequestPath:         c.Request.URL.Path,
		ClientIp:            c.ClientIP(),
	}
	if err := service.WriteIdentityAuditEvent(ev); err != nil {
		common.SysError("write credential rate limit audit error: " + err.Error())
	}
}
