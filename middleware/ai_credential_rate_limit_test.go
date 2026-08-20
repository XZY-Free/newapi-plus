package middleware

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
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
