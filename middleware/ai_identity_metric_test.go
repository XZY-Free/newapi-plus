package middleware

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decisionCapture 采集 AIIdentityAuth 在治理决策点登记的身份验证处置。
type decisionCapture struct {
	decision *identityDecision
	has      bool
}

// newDecisionRouter 构造 TokenAuth(设置 token_id) → 捕获 → AIIdentityAuth → 终态 的 router。
// 捕获中间件在 c.Next()（执行 AIIdentityAuth，含其 abort 与继续两种情况）返回后读取
// ContextKeyIdentityDecision，从而在 ENFORCE 阻断（不执行终态）时也能断言 REJECTED。
func newDecisionRouter(t *testing.T, tokenID int, cap *decisionCapture) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		if tokenID > 0 {
			c.Set("token_id", tokenID)
		}
	})
	r.Use(func(c *gin.Context) {
		c.Next()
		if v, ok := c.Get(string(constant.ContextKeyIdentityDecision)); ok {
			if d, ok := v.(*identityDecision); ok && d != nil {
				cap.decision = d
				cap.has = true
			}
		}
	})
	r.Use(AIIdentityAuth())
	r.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return r
}

// TestVerificationResultForContext 冻结 P1 #2 三态处置语义：
// 强身份验证失败但 audit 模式允许继续 → 一律 UNVERIFIED，绝不因 FailureReason 存在而 REJECTED。
func TestVerificationResultForContext(t *testing.T) {
	// audit 失败 + 继续：UNVERIFIED，且 reason_code 保留。
	ctx := &constant.TrustedAttributionContext{
		IdentityVerified: false,
		FailureReason:    constant.ReasonCodeSignatureInvalid,
	}
	assert.Equal(t, constant.IdentityAuditResultUnverified, verificationResultForContext(ctx), "audit 失败+继续必须 UNVERIFIED，绝不 REJECTED")
	assert.Equal(t, constant.ReasonCodeSignatureInvalid, ctx.FailureReason, "reason_code 必须保留")

	// 无失败原因也未验证 → UNVERIFIED。
	assert.Equal(t, constant.IdentityAuditResultUnverified, verificationResultForContext(&constant.TrustedAttributionContext{IdentityVerified: false}))

	// 身份已建立 → VERIFIED。
	assert.Equal(t, constant.IdentityAuditResultVerified, verificationResultForContext(&constant.TrustedAttributionContext{IdentityVerified: true}))

	// nil（未做治理判定）→ 空串（defer 不计数）。
	assert.Equal(t, "", verificationResultForContext(nil))
}

// TestIdentityMetricDecisionCaptured 在每个请求的治理决策点采集恰一次的处置：
//   - audit 强身份失败 + 继续 → UNVERIFIED（携带 SIGNATURE_INVALID）
//   - enforce 强身份失败 → REJECTED（携带 SIGNATURE_INVALID），HTTP 403 阻断
//   - audit/enforce 合法签名 → VERIFIED
//   - 非法模式（消费路径）→ REJECTED（ATTRIBUTION_MODE_INVALID），HTTP 503
//   - disabled 模式 → 不做治理判定，不产生处置（无重复/无空计）
func TestIdentityMetricDecisionCaptured(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	ctxPayload := `{"root_app_id":"` + boundAppCode(t, tokenID) + `","root_run_id":"r-metric"}`

	t.Run("audit_bad_signature_unverified_continue", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
		require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
		cap := &decisionCapture{}
		r := newDecisionRouter(t, tokenID, cap)
		fn := signedHeaderFnNonce(t, snap, secret, ctxPayload, "metric-audit-bad-abcdefghij")
		bad := func(req *http.Request) {
			fn(req)
			req.Header.Set(constant.AIHeaderSignature, "0000000000000000000000000000000000000000000000000000000000000000")
		}
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", bad)
		require.Equal(t, http.StatusNoContent, rec.Code, "audit 失败必须降级继续(204)")
		require.True(t, cap.has, "audit 失败继续必须登记处置")
		assert.Equal(t, constant.IdentityAuditResultUnverified, cap.decision.result, "audit 失败+继续必须 UNVERIFIED，绝不 REJECTED")
		assert.Equal(t, constant.ReasonCodeSignatureInvalid, cap.decision.reasonCode)
	})

	t.Run("enforce_bad_signature_rejected_blocked", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
		require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
		cap := &decisionCapture{}
		r := newDecisionRouter(t, tokenID, cap)
		fn := signedHeaderFnNonce(t, snap, secret, ctxPayload, "metric-enforce-bad-abcdefghi")
		bad := func(req *http.Request) {
			fn(req)
			req.Header.Set(constant.AIHeaderSignature, "0000000000000000000000000000000000000000000000000000000000000000")
		}
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", bad)
		require.Equal(t, http.StatusForbidden, rec.Code, "enforce 强身份失败必须 403 阻断")
		require.True(t, cap.has, "enforce 阻断请求也必须登记处置（此前 ENFORCE 阻断丢失指标）")
		assert.Equal(t, constant.IdentityAuditResultRejected, cap.decision.result)
		assert.Equal(t, constant.ReasonCodeSignatureInvalid, cap.decision.reasonCode)
	})

	for _, mode := range []string{constant.AttributionModeAudit, constant.AttributionModeEnforce} {
		t.Run("valid_signed_"+mode+"_verified", func(t *testing.T) {
			t.Setenv(constant.AttributionModeEnv, mode)
			require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
			cap := &decisionCapture{}
			r := newDecisionRouter(t, tokenID, cap)
			fn := signedHeaderFnNonce(t, snap, secret, ctxPayload, "metric-valid-"+mode+"-abcdefgh")
			rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", fn)
			require.Equal(t, http.StatusNoContent, rec.Code)
			require.True(t, cap.has)
			assert.Equal(t, constant.IdentityAuditResultVerified, cap.decision.result, "身份建立必须 VERIFIED")
			assert.Empty(t, cap.decision.reasonCode)
		})
	}
}

// TestIdentityMetricDisabledAndInvalidMode 验证 disabled 不做治理判定（不产生处置），
// 非法模式在消费路径 fail-closed 503 并登记 REJECTED(ATTRIBUTION_MODE_INVALID)。
func TestIdentityMetricDisabledAndInvalidMode(t *testing.T) {
	setupAIMiddlewareEnv(t) // 非法模式 fail-closed 503 会落审计事件，需要可用 DB
	t.Run("disabled_no_decision", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeDisabled)
		cap := &decisionCapture{}
		r := newDecisionRouter(t, 0, cap)
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.False(t, cap.has, "disabled 模式不做治理判定，不得产生身份验证指标")
	})

	t.Run("invalid_mode_consumption_rejected", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, "bogus")
		cap := &decisionCapture{}
		r := newDecisionRouter(t, 0, cap)
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code, "非法模式消费路径必须 fail-closed 503")
		require.True(t, cap.has)
		assert.Equal(t, constant.IdentityAuditResultRejected, cap.decision.result)
		assert.Equal(t, constant.ReasonCodeAttributionModeInvalid, cap.decision.reasonCode)
	})
}
