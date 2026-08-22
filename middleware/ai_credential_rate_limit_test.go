package middleware

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRateLimitRouter 构造 TokenAuth → AIIdentityAuth → AICredentialRateLimit → 终态(204) 的 router。
// 使用 STATIC/PRINCIPAL profile（无需签名，ProfileID 稳定可用）。
func newRateLimitRouter(t *testing.T, tokenID int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		// RequestId 每请求唯一：限流 Lua 按 member 计数，member 复用会导致 ZADD 覆盖而非新增。
		c.Set(common.RequestIdKey, "rl-req-"+strconv.Itoa(int(atomic.AddInt64(&aiReqSeq, 1))))
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.Use(AICredentialRateLimit())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return r
}

func rlPatch(enabled bool, window, max int) service.IdentityProfilePatch {
	return service.IdentityProfilePatch{RateLimitEnabled: &enabled, RateLimitWindowSeconds: &window, RateLimitMaxRequests: &max}
}

func TestAICredentialRateLimitDisabledModeSkips(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeDisabled)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 2))

	r := newRateLimitRouter(t, tokenID)
	for i := 0; i < 5; i++ {
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusNoContent, rec.Code, "disabled 模式不限流")
	}
}

func TestAICredentialRateLimitExceedsEnforce(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 2))

	r := newRateLimitRouter(t, tokenID)
	require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
	require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
	third := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusTooManyRequests, third.Code, "同 Profile 达到阈值 429")
	require.Contains(t, third.Body.String(), constant.AICredentialRateLimitExceeded)
}

func TestAICredentialRateLimitExceedsAudit(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 2))

	r := newRateLimitRouter(t, tokenID)
	require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
	require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
	third := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusTooManyRequests, third.Code, "audit 超阈值同样 429（Provider 不收到请求）")
}

func TestAICredentialRateLimitProfileDisabledLimiting(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})

	r := newRateLimitRouter(t, tokenID)
	for i := 0; i < 10; i++ {
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusNoContent, rec.Code, "未启用限流的 Profile 不限流")
	}
}

func TestAICredentialRateLimitStoreUnavailable(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 10))

	t.Run("enforce 503 fail closed", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
		common.RDB = nil
		r := newRateLimitRouter(t, tokenID)
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		require.Contains(t, rec.Body.String(), constant.AICredentialRateLimitStoreUnavailable)
	})

	t.Run("audit allows", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
		common.RDB = nil
		r := newRateLimitRouter(t, tokenID)
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusNoContent, rec.Code, "audit 存储不可用放行")
	})
}

// D.2：CREDENTIAL_RATE_LIMIT_EXCEEDED 在 audit/enforce 两个模式都实际 429 阻断（Provider
// 收不到请求），故审计 result 一律 REJECTED，不得写成 UNVERIFIED。
func TestAICredentialRateLimitExceededDispositionResult(t *testing.T) {
	for _, mode := range []string{constant.AttributionModeAudit, constant.AttributionModeEnforce} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(constant.AttributionModeEnv, mode)
			setupAIMiddlewareEnv(t)
			tokenID, _ := createAIMiddlewareProfile(t,
				constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
				constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 2))
			require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)

			r := newRateLimitRouter(t, tokenID)
			require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
			require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
			third := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
			require.Equal(t, http.StatusTooManyRequests, third.Code, "%s 超阈值 429", mode)

			evs := listIdentityAuditEvents(t)
			require.Len(t, evs, 1, "%s 超阈值恰好一条审计事件", mode)
			assert.Equal(t, constant.IdentityAuditResultRejected, evs[0].Result, "%s 超阈值 429 阻断 → REJECTED", mode)
			assert.Equal(t, constant.ReasonCodeCredentialRateLimitExceeded, evs[0].ReasonCode)
		})
	}
}

// D.2：CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE 依实际处置定 result：
// audit 放行 → UNVERIFIED；enforce 503 fail-closed → REJECTED。
func TestAICredentialRateLimitStoreUnavailableDispositionResult(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 10))

	t.Run("audit allows → UNVERIFIED", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
		common.RDB = nil
		require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
		r := newRateLimitRouter(t, tokenID)
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusNoContent, rec.Code, "audit 存储不可用放行")
		evs := listIdentityAuditEvents(t)
		require.Len(t, evs, 1)
		assert.Equal(t, constant.IdentityAuditResultUnverified, evs[0].Result)
		assert.Equal(t, constant.ReasonCodeCredentialRateLimitStoreUnavailable, evs[0].ReasonCode)
	})
	t.Run("enforce 503 → REJECTED", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
		common.RDB = nil
		require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
		r := newRateLimitRouter(t, tokenID)
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code, "enforce 存储不可用 503")
		evs := listIdentityAuditEvents(t)
		require.Len(t, evs, 1)
		assert.Equal(t, constant.IdentityAuditResultRejected, evs[0].Result)
		assert.Equal(t, constant.ReasonCodeCredentialRateLimitStoreUnavailable, evs[0].ReasonCode)
	})
}

func TestAICredentialRateLimitDifferentProfilesIndependent(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenA, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 1))
	tokenB, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 1))

	ra := newRateLimitRouter(t, tokenA)
	rb := newRateLimitRouter(t, tokenB)
	// ra 先耗尽自己的桶（max=1：首请求即满）。
	require.Equal(t, http.StatusNoContent, performAIRequest(ra, http.MethodPost, "/v1/chat/completions", nil).Code)
	require.Equal(t, http.StatusTooManyRequests, performAIRequest(ra, http.MethodPost, "/v1/chat/completions", nil).Code)
	// ra 已 429，但 rb 仍可发起请求（各 Profile 桶独立）。
	require.Equal(t, http.StatusNoContent, performAIRequest(rb, http.MethodPost, "/v1/chat/completions", nil).Code)
	require.Equal(t, http.StatusTooManyRequests, performAIRequest(rb, http.MethodPost, "/v1/chat/completions", nil).Code)
}
