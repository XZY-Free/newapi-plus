package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countIdentityAuditEvents 返回当前主库身份审计事件总数。
func countIdentityAuditEvents(t *testing.T) int64 {
	t.Helper()
	var n int64
	require.NoError(t, model.DB.Model(&model.AIIdentityAuditEvent{}).Count(&n).Error)
	return n
}

// listIdentityAuditEvents 返回当前主库全部身份审计事件（升序）。
func listIdentityAuditEvents(t *testing.T) []model.AIIdentityAuditEvent {
	t.Helper()
	var evs []model.AIIdentityAuditEvent
	require.NoError(t, model.DB.Order("id asc").Find(&evs).Error)
	return evs
}

// terminalCounter 统计终态（Provider 收到请求）执行次数。
type terminalCounter struct {
	hits int
}

// newAuditCountingRouter 构造 TokenAuth → AIIdentityAuth → 终态(204, 计数) 的 router。
// 不设置 request_id，用于验证降级/重放是否落审计、是否执行终态。
func newAuditCountingRouter(t *testing.T, tokenID int, tc *terminalCounter) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		tc.hits++
		c.Status(http.StatusNoContent)
	})
	return r
}

// newReplayModeRouter 构造 TokenAuth → AIIdentityAuth → 终态(204, 计数 + 采集归因) 的 router。
// 同时统计终态命中数并采集降级/放行后的可信归因，供重放模式语义断言使用。
func newReplayModeRouter(t *testing.T, tokenID int, tc *terminalCounter, capture *aiCapture) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		if tc != nil {
			tc.hits++
		}
		if a, ok := common.GetTrustedAttribution(c); ok {
			capture.attribution = a
			capture.hasAttribution = true
		}
		c.Status(http.StatusNoContent)
	})
	return r
}

// --- 验收 #1 / #11：audit 强身份失败降级放行并落 UNVERIFIED 审计；enforce/replay REJECTED ---

func TestAIIdentityAuthAuditDegradeTerminalAndAudit(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)

	cases := []struct {
		name    string
		payload string
		reason  string
		mutate  func(*http.Request)
	}{
		{"bad signature", `{"root_app_id":"` + boundAppCode(t, tokenID) + `","root_run_id":"r1"}`, constant.ReasonCodeSignatureInvalid,
			func(req *http.Request) {
				req.Header.Set(constant.AIHeaderSignature, strings.Repeat("0", 64))
			}},
		{"missing context", `{"root_app_id":"` + boundAppCode(t, tokenID) + `","root_run_id":"r1"}`, constant.ReasonCodeContextRequired,
			func(req *http.Request) {
				req.Header.Del(constant.AIHeaderContext)
			}},
		{"app not bound", `{"root_app_id":"unbound","root_run_id":"r1"}`, constant.ReasonCodeAppNotBound, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
			// 每个子测试独立清空审计表，保证计数/断言隔离。
			require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
			before := countIdentityAuditEvents(t)
			tcObj := &terminalCounter{}
			r := newAuditCountingRouter(t, tokenID, tcObj)
			fn := signedHeaderFn(t, snap, secret, tc.payload)
			req := performAIRequest(r, http.MethodPost, "/v1/chat/completions", func(hr *http.Request) {
				fn(hr)
				if tc.mutate != nil {
					tc.mutate(hr)
				}
			})
			require.Equal(t, http.StatusNoContent, req.Code, "audit 确定性失败必须降级放行并执行终态")
			require.Equal(t, 1, tcObj.hits, "audit 降级必须执行终态（Provider 收到请求）")
			require.EqualValues(t, before+1, countIdentityAuditEvents(t), "audit 降级必须写一条审计事件")

			evs := listIdentityAuditEvents(t)
			last := evs[len(evs)-1]
			assert.Equal(t, constant.IdentityAuditResultUnverified, last.Result, "audit 降级结果 UNVERIFIED")
			assert.Equal(t, tc.reason, last.ReasonCode)
		})
	}
}

// signedHeaderFnNonce 返回以指定 nonce 签名的 header 设置函数（nonce 须唯一，避免跨子测试
// 复用同一 nonce 被 Redis 判为重放）。
func signedHeaderFnNonce(t *testing.T, snap *types.IdentitySnapshot, secret []byte, ctxPayload, nonce string) func(*http.Request) {
	t.Helper()
	encoded := b64urlMW(ctxPayload)
	timestamp := strconv.FormatInt(common.GetTimestamp(), 10)
	canonical := service.BuildCanonicalString("POST", "/v1/chat/completions", timestamp, nonce, snap.SigningKeys[0].KeyId, encoded)
	sig := service.HMACSHA256Hex(secret, canonical)
	return func(req *http.Request) {
		req.Header.Set(constant.AIHeaderContextVersion, constant.AttributionContextVersion)
		req.Header.Set(constant.AIHeaderContext, encoded)
		req.Header.Set(constant.AIHeaderTimestamp, timestamp)
		req.Header.Set(constant.AIHeaderNonce, nonce)
		req.Header.Set(constant.AIHeaderKeyId, snap.SigningKeys[0].KeyId)
		req.Header.Set(constant.AIHeaderSignature, sig)
	}
}

// TestAIIdentityAuthReplayModeSemantics 验收 V1.1 7.8：已消费 nonce 的强身份请求再次发送
// 必须按模式区分——AUDIT 不得把重放当作硬拒绝，而是降级继续并写 UNVERIFIED REPLAY_DETECTED；
// ENFORCE 仍拒绝并写 REJECTED REPLAY_DETECTED。
func TestAIIdentityAuthReplayModeSemantics(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	ctxPayload := `{"root_app_id":"` + boundAppCode(t, tokenID) + `","root_run_id":"r-replay"}`

	for _, mode := range []string{constant.AttributionModeAudit, constant.AttributionModeEnforce} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(constant.AttributionModeEnv, mode)
			require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
			tc := &terminalCounter{}
			capture := &aiCapture{}
			r := newReplayModeRouter(t, tokenID, tc, capture)
			// 每个 mode 使用唯一 nonce，避免跨 mode 复用导致误判重放。
			fn := signedHeaderFnNonce(t, snap, secret, ctxPayload, "unique-"+mode+"-abcdefghijklmnop")
			// 第一个请求成功（claim nonce 并执行终态）。
			first := performAIRequest(r, http.MethodPost, "/v1/chat/completions", fn)
			require.Equal(t, http.StatusNoContent, first.Code, "首次正确签名请求必须成功")
			require.Equal(t, 1, tc.hits)
			// 清理首个请求的审计行，聚焦重放本身的行为。
			require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)

			// 第二个请求：相同 nonce/signature → 重放。
			rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", fn)

			if mode == constant.AttributionModeAudit {
				// V1.1 7.8：AUDIT 重放必须降级继续，不得拒绝。
				require.Equal(t, http.StatusNoContent, rec.Code, "AUDIT 重放必须降级放行(204)")
				require.Equal(t, 2, tc.hits, "AUDIT 重放必须继续执行终态")
				require.True(t, capture.hasAttribution, "AUDIT 重放降级必须生成可信归因")
				ctx := capture.attribution
				assert.True(t, ctx.CredentialVerified, "AUDIT 重放降级保留 token/profile 静态事实 credential_verified=true")
				assert.False(t, ctx.IdentityVerified, "AUDIT 重放降级 identity_verified=false")
				assert.False(t, ctx.ClientVerified, "AUDIT 重放降级 client_verified=false")
				assert.Equal(t, "", ctx.RootAppID, "AUDIT 重放降级不得采纳客户端 root_app")
				assert.Equal(t, constant.ReasonCodeReplayDetected, ctx.FailureReason)
				// 恰好一条 UNVERIFIED REPLAY_DETECTED 事件。
				evs := listIdentityAuditEvents(t)
				require.Len(t, evs, 1, "AUDIT 重放必须恰好写一条审计事件")
				assert.Equal(t, constant.IdentityAuditResultUnverified, evs[0].Result, "AUDIT 重放审计结果 UNVERIFIED")
				assert.Equal(t, constant.ReasonCodeReplayDetected, evs[0].ReasonCode)
			} else {
				// ENFORCE：仍拒绝重放，不得执行终态。
				require.Equal(t, http.StatusForbidden, rec.Code, "ENFORCE 重放必须拒绝(403)")
				require.Contains(t, rec.Body.String(), constant.AIIdentityReplayDetected)
				require.Equal(t, 1, tc.hits, "ENFORCE 重放不得执行终态")
				evs := listIdentityAuditEvents(t)
				require.Len(t, evs, 1, "ENFORCE 重放必须恰好写一条审计事件")
				assert.Equal(t, constant.IdentityAuditResultRejected, evs[0].Result, "ENFORCE 重放审计结果 REJECTED")
				assert.Equal(t, constant.ReasonCodeReplayDetected, evs[0].ReasonCode)
			}
		})
	}
}

func TestAIIdentityAuthEnforceFailureRejectedAuditResult(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)

	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
	tc := &terminalCounter{}
	r := newAuditCountingRouter(t, tokenID, tc)
	fn := signedHeaderFn(t, snap, secret, `{"root_app_id":"`+boundAppCode(t, tokenID)+`","root_run_id":"r-enforce"}`)
	bad := func(req *http.Request) {
		fn(req)
		req.Header.Set(constant.AIHeaderSignature, strings.Repeat("0", 64))
	}
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", bad)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, 0, tc.hits, "enforce 拒绝不得执行终态")
	evs := listIdentityAuditEvents(t)
	require.Len(t, evs, 1)
	assert.Equal(t, constant.IdentityAuditResultRejected, evs[0].Result)
	assert.Equal(t, constant.ReasonCodeSignatureInvalid, evs[0].ReasonCode)
}

// --- #11：审计事件 JSON 不得携带 context/signature/nonce/secret/token key ---

func TestIdentityAuditEventJSONExcludesSensitiveFields(t *testing.T) {
	ev := &model.AIIdentityAuditEvent{
		RequestId: "req", TokenId: 1, ProfileId: 2, Result: constant.IdentityAuditResultRejected,
		ReasonCode: constant.ReasonCodeSignatureInvalid, HttpMethod: "POST", RequestPath: "/v1/chat/completions",
	}
	raw, err := common.Marshal(ev)
	require.NoError(t, err)
	s := string(raw)
	// 只检查“字段键”形式的敏感数据是否存在（不 lower 整个串，避免 ReasonCode 值
	// SIGNATURE_INVALID 等大写代码误报）。
	for _, forbiddenKey := range []string{
		`"api_key"`, `"api-key"`, `"token_key"`, `"signing_key_id"`, `"secret"`,
		`"nonce"`, `"signature"`, `"context"`, `"key_id"`, `"x-ai-`,
	} {
		assert.False(t, strings.Contains(s, forbiddenKey), "审计事件不得含敏感字段键: %s", forbiddenKey)
	}
	// 必须含安全事实字段。
	assert.Contains(t, s, `"result"`)
	assert.Contains(t, s, `"reason_code"`)
}

// --- #5：TrustedContext JSON 契约 ---

func TestTrustedAttributionContextJSONContract(t *testing.T) {
	ctx := &constant.TrustedAttributionContext{
		TokenID: 1, ProfileID: 2, IdentityMode: constant.IdentityModeStatic,
		AttributionTarget: constant.AttributionTargetApplication,
		RootAppID:         "hr", ApplicationBusinessDomainID: 7, ApplicationBusinessDomainCode: "fin",
		ApplicationBusinessDomainName: "财务", OwnerTeamID: 9, OwnerTeamCode: "ai", OwnerTeamName: "AI组",
		SigningKeyID: "key-1",
	}
	raw, err := common.Marshal(ctx)
	require.NoError(t, err)
	s := string(raw)
	// 7.13 json tag：attribution_target_type 而非 attribution_target。
	assert.Contains(t, s, `"attribution_target_type":"APPLICATION"`)
	// 应用业务领域字段必须为 application_business_domain_*。
	assert.Contains(t, s, `"application_business_domain_id":7`)
	assert.Contains(t, s, `"application_business_domain_code":"fin"`)
	assert.Contains(t, s, `"application_business_domain_name":"财务"`)
	// 不得出现 numeric application_id 字段。
	assert.NotContains(t, s, `"application_id"`)
	assert.NotContains(t, s, `"app_business_domain_id"`)
}

// --- #2：AICredentialRateLimit 对非法 mode 独立 503 ---

func TestAICredentialRateLimitInvalidModeIndependent503(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, "bogus")
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 10))

	r := newRateLimitRouter(t, tokenID)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, "AICredentialRateLimit 对非法 mode 必须独立 503，不视为 disabled")
	require.Contains(t, rec.Body.String(), constant.AIIdentityAttributionModeInvalid)
}

// --- #6：audit 静态降级 ---

func TestAIIdentityAuthProfileMissingAuditTokenOnlyContext(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
	setupAIMiddlewareEnv(t)
	tokenID := createAIMiddlewareToken(t) // 有 token 无 profile

	require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, capture.hasAttribution)
	ctx := capture.attribution
	assert.Equal(t, tokenID, ctx.TokenID)
	assert.Equal(t, 0, ctx.ProfileID, "profile missing 仅 token-only 降级")
	// 7.8 AUDIT 冻结：TokenAuth 已成功（token_id>0）→ credential_verified 必须为 true，
	// 即使缺失/无法解析企业身份（Profile），也只降级 identity/client 验证，不得抹掉凭证可信事实。
	assert.True(t, ctx.CredentialVerified, "TokenAuth 已提供 token_id 后 token-only 降级必须保留 credential_verified=true")
	assert.False(t, ctx.IdentityVerified, "profile missing 不得冒充身份已验证")
	assert.False(t, ctx.ClientVerified, "缺失/无效企业身份不得暗示 client_verified=true")
	assert.Equal(t, constant.IdentitySourceToken, ctx.IdentitySource)
	// 审计 UNVERIFIED / PROFILE_REQUIRED。
	evs := listIdentityAuditEvents(t)
	require.Len(t, evs, 1)
	assert.Equal(t, constant.IdentityAuditResultUnverified, evs[0].Result)
	assert.Equal(t, constant.ReasonCodeProfileRequired, evs[0].ReasonCode)
}

func TestAIIdentityAuthProfileDisabledAuditDegraded(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	enabled := false
	_, err = service.UpdateIdentityProfile(&service.IdentityProfilePatch{Id: snap.ProfileID, Enabled: &enabled})
	require.NoError(t, err)

	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusNoContent, rec.Code, "audit profile disabled 降级放行")
	require.True(t, capture.hasAttribution)
	ctx := capture.attribution
	assert.False(t, ctx.IdentityVerified, "降级 identity_verified=false")
	assert.Equal(t, constant.ReasonCodeProfileDisabled, ctx.FailureReason)
	// 保留可确定的 Token/Profile 静态事实。
	assert.Equal(t, tokenID, ctx.TokenID)
	assert.Equal(t, snap.ProfileID, ctx.ProfileID)
}

func TestAIIdentityAuthStaticEntityDisabledAuditClearsAttribution(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	enabled := false
	_, err = service.UpdatePrincipal(snap.PrincipalID, "张三", snap.UsageDomainID, snap.UsageTeamID, &enabled)
	require.NoError(t, err)

	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusNoContent, rec.Code, "audit STATIC 实体 disabled 降级放行(204)，不拒绝")
	require.True(t, capture.hasAttribution)
	ctx := capture.attribution
	// 清除被停用 principal 相关归因。
	assert.Equal(t, 0, ctx.PrincipalID, "principal disabled 必须清除 principal 归因")
	assert.Equal(t, "", ctx.PrincipalCode)
	assert.Equal(t, 0, ctx.UsageTeamID)
	assert.False(t, ctx.IdentityVerified)
	assert.False(t, ctx.ClientVerified)
	assert.Equal(t, constant.ReasonCodePrincipalDisabled, ctx.FailureReason)
}

// --- #8：rate-limit member 唯一性 + 429 不执行终态 + Redis 审计 reason ---

// newNoRequestIDRateLimitRouter 构造不设置 request_id 的限流 router，验证 member 缺失时仍唯一计数。
func newNoRequestIDRateLimitRouter(t *testing.T, tokenID int, tc *terminalCounter) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.Use(AICredentialRateLimit())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		tc.hits++
		c.Status(http.StatusNoContent)
	})
	return r
}

func TestAICredentialRateLimitUniqueMemberWithoutRequestID(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 1))

	tc := &terminalCounter{}
	r := newNoRequestIDRateLimitRouter(t, tokenID, tc)
	// max=1：首个请求放行并执行终态；第二个请求即使 request_id 缺失也必须独立计数 → 429。
	require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
	require.Equal(t, 1, tc.hits)
	second := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusTooManyRequests, second.Code, "request_id 缺失时 member 必须唯一，否则 ZADD 覆盖导致不计数")
	require.Equal(t, 1, tc.hits, "429 不得执行终态（Provider 不收到请求）")
}

// TestAICredentialRateLimitSameRequestIDStillCounts 回归：同一 request_id 重复时，每请求仍须
// 计为滑动窗口内独立成员（member 唯一，不得因 ZADD 同 member 覆盖而丢失计数）。
// max=2 时前两个请求放行并执行终态，第三个请求必须 429 且终态命中数保持 2。
func TestAICredentialRateLimitSameRequestIDStillCounts(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 2))

	tc := &terminalCounter{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "same-request-id") // 相同 request_id 也须按请求唯一计数
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.Use(AICredentialRateLimit())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		tc.hits++
		c.Status(http.StatusNoContent)
	})

	require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
	require.Equal(t, 1, tc.hits, "第一个请求放行并执行终态")
	require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
	require.Equal(t, 2, tc.hits, "第二个请求放行并执行终态")
	third := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusTooManyRequests, third.Code, "相同 request_id 重复时第三个请求必须 429（每请求独立成员计数）")
	require.Equal(t, 2, tc.hits, "429 不得执行终态（Provider 不收到请求）")
}

func TestAICredentialRateLimitAuditWritesReasonToRedis(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, rlPatch(true, 60, 1))

	require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
	r := newRateLimitRouter(t, tokenID)
	require.Equal(t, http.StatusNoContent, performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil).Code)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	evs := listIdentityAuditEvents(t)
	require.Len(t, evs, 1, "429 必须落一条凭证限流审计事件")
	assert.Equal(t, constant.ReasonCodeCredentialRateLimitExceeded, evs[0].ReasonCode)
	// D.2 处置语义：CREDENTIAL_RATE_LIMIT_EXCEEDED 两个模式都实际 429 阻断 → REJECTED。
	assert.Equal(t, constant.IdentityAuditResultRejected, evs[0].Result)
}

// --- #12：Kling/Jimeng converter 外部 canonical 路径 + GetResult 跳过 ---

// signExternalRequest 以指定外部 method/path 计算签名并写入六个 AI Header。
func signExternalRequest(t *testing.T, snap *types.IdentitySnapshot, secret []byte, method, path, ctxPayload, nonce string) func(*http.Request) {
	t.Helper()
	encoded := b64urlMW(ctxPayload)
	timestamp := strconv.FormatInt(common.GetTimestamp(), 10)
	canonical := service.BuildCanonicalString(method, path, timestamp, nonce, snap.SigningKeys[0].KeyId, encoded)
	sig := service.HMACSHA256Hex(secret, canonical)
	return func(req *http.Request) {
		req.Header.Set(constant.AIHeaderContextVersion, constant.AttributionContextVersion)
		req.Header.Set(constant.AIHeaderContext, encoded)
		req.Header.Set(constant.AIHeaderTimestamp, timestamp)
		req.Header.Set(constant.AIHeaderNonce, nonce)
		req.Header.Set(constant.AIHeaderKeyId, snap.SigningKeys[0].KeyId)
		req.Header.Set(constant.AIHeaderSignature, sig)
	}
}

// newKlingRouter 构造 KlingRequestConvert → TokenAuth → AIIdentityAuth → 终态(计数)。
func newKlingRouter(t *testing.T, tokenID int, capture *aiCapture, tc *terminalCounter) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(KlingRequestConvert())
	r.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "req-kling")
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.POST("/kling/v1/videos/text2video", func(c *gin.Context) {
		if tc != nil {
			tc.hits++
		}
		if a, ok := common.GetTrustedAttribution(c); ok {
			capture.attribution = a
			capture.hasAttribution = true
		}
		c.Status(http.StatusNoContent)
	})
	return r
}

func TestAIIdentityAuthKlingExternalPathSignatureSucceeds(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)

	capture := &aiCapture{}
	tc := &terminalCounter{}
	r := newKlingRouter(t, tokenID, capture, tc)
	// 客户端以“外部入站路径”/kling/v1/videos/text2video 签名；converter 已把该路径
	// 存入 context，AIIdentityAuth 签名验证使用外部路径而非改写后的 /v1/video/generations。
	fn := signExternalRequest(t, snap, secret, http.MethodPost, "/kling/v1/videos/text2video",
		`{"root_app_id":"`+boundAppCode(t, tokenID)+`","root_run_id":"r-kling"}`, "kling-nonce-abcdefghijklmnop")

	req := httptest.NewRequest(http.MethodPost, "/kling/v1/videos/text2video", strings.NewReader(`{"model":"m","prompt":"p"}`))
	fn(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, "Kling 外部路径签名必须验证通过并放行")
	require.Equal(t, 1, tc.hits)
	require.True(t, capture.hasAttribution)
	assert.True(t, capture.attribution.IdentityVerified)
	assert.Equal(t, boundAppCode(t, tokenID), capture.attribution.RootAppID)
}

// newJimengRouter 构造 JimengRequestConvert → TokenAuth → AIIdentityAuth → 终态(计数)。
func newJimengRouter(t *testing.T, tokenID int, tc *terminalCounter) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(JimengRequestConvert())
	r.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "req-jimeng")
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.POST("/jimeng/", func(c *gin.Context) {
		if tc != nil {
			tc.hits++
		}
		c.Status(http.StatusNoContent)
	})
	return r
}

func TestAIIdentityAuthJimengGetResultSkips(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	// GetResult 是查询（converter 改写为 GET + /v1/video/generations/{taskId}），
	// enforce 下用无 profile 的 token 也能通过，证明未做消费身份验证。
	tokenID := createAIMiddlewareToken(t)

	tc := &terminalCounter{}
	r := newJimengRouter(t, tokenID, tc)
	req := httptest.NewRequest(http.MethodPost, "/jimeng/?Action=CVSync2AsyncGetResult&Version=2022-08-31",
		strings.NewReader(`{"req_key":"m","task_id":"task-1","prompt":"p"}`))
	req.Header.Set("Content-Type", "application/json")
	setAllAIHeaders(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, "Jimeng GetResult 查询在 enforce 下必须放行")
	require.Equal(t, 1, tc.hits)
}

// --- #9：MJ classifier 在 middleware 端到端（真实 /:mode/mj 变体） ---

func TestAIIdentityAuthSkipsModeMJFetchAndUpload(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	// 无 profile 的 token 也能通过 skip 端点，证明未做消费身份验证。
	tokenID := createAIMiddlewareToken(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "req-mj-skip")
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	for _, path := range []string{
		"/enforce/mj/task/list-by-condition",
		"/mj/submit/upload-discord-images",
		"/enforce/mj/submit/upload-discord-images",
	} {
		r.POST(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	}

	for _, path := range []string{
		"/enforce/mj/task/list-by-condition",
		"/mj/submit/upload-discord-images",
		"/enforce/mj/submit/upload-discord-images",
	} {
		rec := performAIRequest(r, http.MethodPost, path, setAllAIHeaders)
		require.Equal(t, http.StatusNoContent, rec.Code, "纯上传/查询 %s 在 enforce 下也必须放行", path)
	}
}

// TestSnapshotValidityFailureUsesFixedErrorCodes 验收冻结点 K（V1.1 7.19）：运行时结构校验
// 每个冻结非法条件必须映射到各自独立的对外 API error code，而不是统一用
// AI_IDENTITY_PROFILE_DISABLED。
func TestSnapshotValidityFailureUsesFixedErrorCodes(t *testing.T) {
	cases := []struct {
		name     string
		snapshot *types.IdentitySnapshot
		wantCode string
	}{
		{
			name: "invalid attribution target",
			snapshot: &types.IdentitySnapshot{
				AttributionTarget: "bogus",
				IdentityAssurance: constant.IdentityAssuranceCredentialOnly,
			},
			wantCode: "AI_IDENTITY_TARGET_INVALID",
		},
		{
			name: "invalid identity assurance",
			snapshot: &types.IdentitySnapshot{
				AttributionTarget: constant.AttributionTargetPrincipal,
				IdentityAssurance: "bogus",
			},
			wantCode: "AI_IDENTITY_ASSURANCE_INVALID",
		},
		{
			name: "PRINCIPAL target missing principal",
			snapshot: &types.IdentitySnapshot{
				AttributionTarget:   constant.AttributionTargetPrincipal,
				IdentityAssurance:   constant.IdentityAssuranceCredentialOnly,
				PrincipalID:         0,
				PrincipalCode:       "",
				CredentialPurposeID: 2,
				UsageTeamID:         3,
			},
			wantCode: "AI_IDENTITY_PRINCIPAL_REQUIRED",
		},
		{
			name: "PRINCIPAL target missing purpose",
			snapshot: &types.IdentitySnapshot{
				AttributionTarget:   constant.AttributionTargetPrincipal,
				IdentityAssurance:   constant.IdentityAssuranceCredentialOnly,
				PrincipalID:         1,
				PrincipalCode:       "p1",
				CredentialPurposeID: 0,
				UsageTeamID:         3,
			},
			wantCode: "AI_IDENTITY_PURPOSE_REQUIRED",
		},
		{
			name: "PRINCIPAL target missing usage team",
			snapshot: &types.IdentitySnapshot{
				AttributionTarget:   constant.AttributionTargetPrincipal,
				IdentityAssurance:   constant.IdentityAssuranceCredentialOnly,
				PrincipalID:         1,
				PrincipalCode:       "p1",
				CredentialPurposeID: 2,
				UsageTeamID:         0,
			},
			wantCode: "AI_IDENTITY_USAGE_TEAM_INVALID",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failure := snapshotValidityFailure(tc.snapshot)
			require.NotNil(t, failure, "每个冻结非法条件都必须产生结构校验失败")
			assert.Equal(t, tc.wantCode, string(failure.code),
				"对外 API error code 必须为条件专属（V1.1 7.19），不得使用通用 AI_IDENTITY_PROFILE_DISABLED")
		})
	}
}

// TestAIIdentityAuthAuditRecordsClaimedUnboundAppOnlyInAudit 验收：DYNAMIC 请求携带
// 合法编码且合法签名的 context，但 root_app_id 声称一个未绑定却语法合法的 app_code 时，
// AUDIT 模式必须以降级可信归因继续（RootAppID 保持为空），且仅把该合法声明写入
// ai_identity_audit_events.claimed_root_app_id（reason=APP_NOT_BOUND），绝不进入正式归因。
func TestAIIdentityAuthAuditRecordsClaimedUnboundAppOnlyInAudit(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)

	require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)

	// 未绑定但语法合法的 app_code：首字符小写，仅含小写字母/数字/_。
	legalUnbound := "unbound_app_abcdef"
	ctxPayload := `{"root_app_id":"` + legalUnbound + `","root_run_id":"r-unbound"}`
	fn := signedHeaderFnNonce(t, snap, secret, ctxPayload, "unbound-app-nonce-abcdefghijklmnop")

	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", fn)
	require.Equal(t, http.StatusNoContent, rec.Code, "AUDIT 未绑定 app 必须降级放行(204)")

	// 降级可信归因必须保持干净：声明只留在审计，绝不进入正式归因。
	require.True(t, capture.hasAttribution)
	ctx := capture.attribution
	assert.True(t, ctx.CredentialVerified, "TokenAuth 成功后 credential_verified=true")
	assert.False(t, ctx.IdentityVerified, "未绑定 app 不得视为身份已验证")
	assert.False(t, ctx.ClientVerified, "未绑定 app 不得视为客户端已验证")
	assert.Equal(t, "", ctx.RootAppID, "未绑定声明不得进入正式归因 RootAppID（保持空）")
	assert.Equal(t, constant.ReasonCodeAppNotBound, ctx.FailureReason)

	// 恰好一条 UNVERIFIED 审计事件，reason=APP_NOT_BOUND。
	evs := listIdentityAuditEvents(t)
	require.Len(t, evs, 1, "恰好一条 UNVERIFIED 审计事件")
	ev := evs[0]
	assert.Equal(t, constant.IdentityAuditResultUnverified, ev.Result)
	assert.Equal(t, constant.ReasonCodeAppNotBound, ev.ReasonCode)
	assert.Equal(t, legalUnbound, ev.ClaimedRootAppId,
		"合法编码+合法签名的未绑定 root_app_id 必须写入审计 claimed_root_app_id")
}
