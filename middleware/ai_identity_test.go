package middleware

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupAIMiddlewareEnv 迁移治理表 + tokens，设置主库/Redis/主密钥，并注册清理。
func setupAIMiddlewareEnv(t *testing.T) {
	t.Helper()
	prevDB := model.DB
	prevType := common.MainDatabaseType()
	prevRedisEnabled := common.RedisEnabled
	prevRedis := common.RDB
	prevMaster := osGetenv(constant.AIAttributionMasterKeyEnv)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.AIGovernanceModels()...))
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))

	t.Cleanup(func() {
		model.DB = prevDB
		common.SetMainDatabaseType(prevType)
		common.RedisEnabled = prevRedisEnabled
		common.RDB = prevRedis
		setOSEnv(constant.AIAttributionMasterKeyEnv, prevMaster)
	})

	_ = redisServerFor(t)
}

// osGetenv / setOSEnv 封装，便于测试中读取/恢复环境变量。
func osGetenv(k string) string { v, _ := os.LookupEnv(k); return v }

func setOSEnv(k, v string) { os.Setenv(k, v) }

func redisServerFor(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	prevEnabled := common.RedisEnabled
	prevRDB := common.RDB
	srv := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	common.RedisEnabled = true
	common.RDB = rdb
	t.Cleanup(func() {
		_ = rdb.Close()
		common.RedisEnabled = prevEnabled
		common.RDB = prevRDB
	})
	return srv
}

// aiMiddlewareTokenSeq 为测试 token 分配全局唯一 id。快照缓存以 token_id 为键且跨测试存活，
// 而每个测试使用全新内存库（自增 id 会重置），必须用全局唯一 id 避免缓存命中他人数据。
var aiMiddlewareTokenSeq int64 = 100000

// aiMasterDataSeq 为主数据实体分配全局唯一 code，避免跨测试（不同内存库）冲突。
var aiMasterDataSeq int64
var aiReqSeq int64

func createAIMiddlewareToken(t *testing.T) int {
	t.Helper()
	atomic.AddInt64(&aiMiddlewareTokenSeq, 1)
	tk := model.Token{
		Id:             int(aiMiddlewareTokenSeq),
		Key:            "sk-test-" + common.GetRandomString(24),
		UserId:         1,
		Status:         1,
		Name:           "t",
		CreatedTime:    common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&tk).Error)
	return tk.Id
}

// createAIMiddlewareProfile 创建一个已启用的 Profile，可按需生成 DYNAMIC/HYBRID 签名密钥
// 并配置限流。返回 (tokenID, secret)。
func createAIMiddlewareProfile(t *testing.T, mode, target, assurance string, bindApp bool, rl service.IdentityProfilePatch) (int, []byte) {
	t.Helper()
	tokenID := createAIMiddlewareToken(t)
	seq := atomic.AddInt64(&aiMasterDataSeq, 1)

	d, err := service.CreateBusinessDomain("finance_"+strconv.FormatInt(seq, 10), "财务")
	require.NoError(t, err)
	ut, err := service.CreateUsageTeam("finance_digital_"+strconv.FormatInt(seq, 10), "财务数字化组")
	require.NoError(t, err)
	ot, err := service.CreateOwnerTeam("ai_application_"+strconv.FormatInt(seq, 10), "AI应用组")
	require.NoError(t, err)
	p, err := service.CreatePrincipal("zhangsan_"+strconv.FormatInt(seq, 10), "张三", d.Id, ut.Id)
	require.NoError(t, err)
	purpose, err := service.CreateCredentialPurpose("workbuddy_"+strconv.FormatInt(seq, 10), "WorkBuddy", constant.PurposeTypeDesktopClient)
	require.NoError(t, err)
	app, err := service.CreateApplication("hr_assistant_"+strconv.FormatInt(seq, 10), "人力助手", d.Id, ot.Id)
	require.NoError(t, err)

	var appIDs []int
	if bindApp {
		appIDs = []int{app.Id}
	}
	prof := &model.AIIdentityProfile{
		TokenId:               tokenID,
		IdentityMode:          mode,
		AttributionTargetType: target,
		IdentityAssurance:     assurance,
		Environment:           "prod",
	}
	switch {
	case mode == constant.IdentityModeStatic && target == constant.AttributionTargetPrincipal:
		prof.PrincipalId = p.Id
		prof.CredentialPurposeId = purpose.Id
	case mode == constant.IdentityModeStatic:
		// STATIC/APPLICATION 禁止配置 Caller，无需额外字段。
	case mode == constant.IdentityModeDynamic || mode == constant.IdentityModeHybrid:
		prof.CallerId = "workflow-prod"
	}
	created, err := service.CreateIdentityProfile(prof, appIDs)
	require.NoError(t, err)

	if rl.RateLimitEnabled != nil {
		rl.Id = created.Id
		_, err = service.UpdateIdentityProfile(&rl)
		require.NoError(t, err)
	}
	var secret []byte
	if mode == constant.IdentityModeDynamic || mode == constant.IdentityModeHybrid {
		_, display, err := service.GenerateSigningKey(created.Id)
		require.NoError(t, err)
		secret, err = base64.RawURLEncoding.DecodeString(display)
		require.NoError(t, err)
	}
	enabled := true
	_, err = service.UpdateIdentityProfile(&service.IdentityProfilePatch{Id: created.Id, Enabled: &enabled})
	require.NoError(t, err)
	return tokenID, secret
}

// aiCapture 是终态处理器采集的中间件产物。
type aiCapture struct {
	remaining      []string
	attribution    *constant.TrustedAttributionContext
	hasAttribution bool
}

// newAIMiddlewareRouter 构造 TokenAuth(设置 token_id) → AIIdentityAuth → 终态采集的 router。
func newAIMiddlewareRouter(t *testing.T, tokenID int, capture *aiCapture) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "req-"+strconv.FormatInt(atomic.AddInt64(&aiReqSeq, 1), 10))
		if tokenID > 0 {
			c.Set("token_id", tokenID)
		}
	})
	r.Use(AIIdentityAuth())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		for _, h := range constant.AIHeaderNames {
			if c.Request.Header.Get(h) != "" {
				capture.remaining = append(capture.remaining, h)
			}
		}
		if a, ok := common.GetTrustedAttribution(c); ok {
			capture.attribution = a
			capture.hasAttribution = true
		}
		c.Status(http.StatusNoContent)
	})
	return r
}

func setAllAIHeaders(req *http.Request) {
	req.Header.Set(constant.AIHeaderContextVersion, "v1")
	req.Header.Set(constant.AIHeaderContext, "dGVzdA")
	req.Header.Set(constant.AIHeaderTimestamp, "1700000000")
	req.Header.Set(constant.AIHeaderNonce, "abcdefghijklmnopqrstuv")
	req.Header.Set(constant.AIHeaderKeyId, "key-1")
	req.Header.Set(constant.AIHeaderSignature, "0")
}

func performAIRequest(r *gin.Engine, method, path string, headerFn func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if headerFn != nil {
		headerFn(req)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func signedHeaderFn(t *testing.T, snap *types.IdentitySnapshot, secret []byte, ctxPayload string) func(*http.Request) {
	t.Helper()
	encoded := b64urlMW(ctxPayload)
	timestamp := strconv.FormatInt(common.GetTimestamp(), 10)
	nonce := "abcdefghijklmnopqrstuv"
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

func b64urlMW(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

// boundAppCode 返回 Profile 绑定的唯一应用 code（DYNAMIC/HYBRID context 中的 root_app_id）。
func boundAppCode(t *testing.T, tokenID int) string {
	t.Helper()
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.Len(t, snap.Applications, 1)
	return snap.Applications[0].AppCode
}

func intPtr(i int) *int { return &i }

// --- 验收 A：六个 X-AI-* Header 在任意模式下都被删除 ---

func TestAIIdentityAuthRemovesHeadersAllModes(t *testing.T) {
	cases := []struct {
		name string
		mode string
	}{
		{"disabled", constant.AttributionModeDisabled},
		{"audit", constant.AttributionModeAudit},
		{"enforce", constant.AttributionModeEnforce},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(constant.AttributionModeEnv, tc.mode)
			setupAIMiddlewareEnv(t)
			tokenID, _ := createAIMiddlewareProfile(t,
				constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
				constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})

			capture := &aiCapture{}
			r := newAIMiddlewareRouter(t, tokenID, capture)
			rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", setAllAIHeaders)
			require.Equal(t, http.StatusNoContent, rec.Code, "任意模式下身份 Header 都不应阻断请求")
			require.Empty(t, capture.remaining, "六类 X-AI-* Header 必须全部删除: %v", capture.remaining)
		})
	}
}

// --- 模式行为：disabled / 非法值 fail-closed ---

func TestAIIdentityAuthDisabledModeNoAttribution(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeDisabled)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})

	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.False(t, capture.hasAttribution, "disabled 模式不得写入可信归因上下文")
}

func TestAIIdentityAuthModeInvalidConsumptionFailsClosed(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, "bogus")
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})

	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", setAllAIHeaders)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, "非法 AI_ATTRIBUTION_MODE 消费请求必须 503，不回退 disabled")
	require.Contains(t, rec.Body.String(), constant.AIIdentityAttributionModeInvalid)
	require.Empty(t, capture.remaining, "即使 fail-closed 也必须清头")
	require.False(t, capture.hasAttribution, "非法 mode 不得写入可信归因上下文")
}

func TestAIIdentityAuthModeInvalidNonConsumptionContinues(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, "bogus")
	setupAIMiddlewareEnv(t)
	tokenID := createAIMiddlewareToken(t) // 无 profile 的非消费查询也应继续

	capture := &aiCapture{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "req-nonconsume-invalid")
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.GET("/v1/models", func(c *gin.Context) {
		for _, h := range constant.AIHeaderNames {
			if c.Request.Header.Get(h) != "" {
				capture.remaining = append(capture.remaining, h)
			}
		}
		c.Status(http.StatusNoContent)
	})

	rec := performAIRequest(r, http.MethodGet, "/v1/models", setAllAIHeaders)
	require.Equal(t, http.StatusNoContent, rec.Code, "非法 mode 非消费查询必须继续并清头")
	require.Empty(t, capture.remaining)
}

// --- STATIC / PRINCIPAL（验收 E / 7.9） ---

func TestAIIdentityAuthStaticPrincipal(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})

	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	for _, mode := range []string{constant.AttributionModeAudit, constant.AttributionModeEnforce} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(constant.AttributionModeEnv, mode)
			capture := &aiCapture{}
			r := newAIMiddlewareRouter(t, tokenID, capture)
			rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
			require.Equal(t, http.StatusNoContent, rec.Code)
			require.True(t, capture.hasAttribution)
			ctx := capture.attribution
			assert.True(t, ctx.CredentialVerified, "STATIC credential_verified=true")
			assert.True(t, ctx.IdentityVerified, "STATIC identity_verified=true")
			assert.False(t, ctx.ClientVerified, "STATIC client_verified=false")
			assert.Equal(t, constant.IdentitySourceToken, ctx.IdentitySource)
			assert.Equal(t, snap.PrincipalCode, ctx.PrincipalCode, "principal_code 传播")
			assert.Equal(t, snap.CredentialPurposeCode, ctx.CredentialPurposeCode, "purpose_code 传播")
			assert.Equal(t, snap.UsageDomainCode, ctx.UsageBusinessDomainCode, "usage domain 传播")
			assert.Equal(t, "", ctx.RootAppID, "STATIC/PRINCIPAL 不得设置 root_app")
		})
	}
}

func TestAIIdentityAuthStaticPrincipalDisabledEnforce(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})

	// 停用 principal → enforce 403 PRINCIPAL_DISABLED。
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	enabled := false
	_, err = service.UpdatePrincipal(snap.PrincipalID, "张三", snap.UsageDomainID, snap.UsageTeamID, &enabled)
	require.NoError(t, err)

	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), constant.AIIdentityPrincipalDisabled)
}

// --- STATIC / APPLICATION（验收 E / 7.10） ---

func TestAIIdentityAuthStaticApplication(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetApplication,
		constant.IdentityAssuranceCredentialOnly, true, service.IdentityProfilePatch{})

	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, capture.hasAttribution)
	assert.Equal(t, boundAppCode(t, tokenID), capture.attribution.RootAppID, "STATIC/APPLICATION 填充 root_app")
	assert.True(t, capture.attribution.CredentialVerified)
}

// --- DYNAMIC / PLATFORM 强身份（验收 C/D/E） ---

func TestAIIdentityAuthDynamicSuccessEnforce(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})

	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	ctxPayload := `{"root_app_id":"` + boundAppCode(t, tokenID) + `","root_run_id":"run-1","current_execution_id":"e1","execution_type":"chat"}`

	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", signedHeaderFn(t, snap, secret, ctxPayload))
	require.Equal(t, http.StatusNoContent, rec.Code, "合法签名 enforce 必须放行")
	require.True(t, capture.hasAttribution)
	ctx := capture.attribution
	assert.True(t, ctx.CredentialVerified)
	assert.True(t, ctx.ClientVerified, "DYNAMIC client_verified=true")
	assert.True(t, ctx.IdentityVerified)
	assert.Equal(t, constant.IdentitySourceSignedContext, ctx.IdentitySource)
	assert.Equal(t, boundAppCode(t, tokenID), ctx.RootAppID)
	assert.Equal(t, "run-1", ctx.RootRunID)
	assert.Equal(t, "workflow-prod", ctx.CallerID)
}

func TestAIIdentityAuthDynamicBadSignature(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	ctxPayload := `{"root_app_id":"hr_assistant","root_run_id":"run-1"}`

	t.Run("audit degrades unverified", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
		capture := &aiCapture{}
		r := newAIMiddlewareRouter(t, tokenID, capture)
		fn := signedHeaderFn(t, snap, secret, ctxPayload)
		bad := func(req *http.Request) {
			fn(req)
			req.Header.Set(constant.AIHeaderSignature, strings.Repeat("0", 64))
		}
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", bad)
		require.Equal(t, http.StatusNoContent, rec.Code, "audit 坏签名必须降级放行")
		require.True(t, capture.hasAttribution, "audit 降级必须写入可信上下文")
		ctx := capture.attribution
		assert.False(t, ctx.IdentityVerified, "audit 降级 identity_verified=false")
		assert.False(t, ctx.ClientVerified, "audit 降级 client_verified=false")
		assert.Equal(t, constant.ReasonCodeSignatureInvalid, ctx.FailureReason)
		// 不得采纳客户端 app/run/execution/signing_key_id。
		assert.Equal(t, "", ctx.RootAppID)
		assert.Equal(t, "", ctx.RootRunID)
		assert.Equal(t, "", ctx.CurrentExecutionID)
		assert.Equal(t, "", ctx.SigningKeyID)
	})

	t.Run("enforce rejects", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
		capture := &aiCapture{}
		r := newAIMiddlewareRouter(t, tokenID, capture)
		fn := signedHeaderFn(t, snap, secret, ctxPayload)
		bad := func(req *http.Request) {
			fn(req)
			req.Header.Set(constant.AIHeaderSignature, strings.Repeat("0", 64))
		}
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", bad)
		require.Equal(t, http.StatusForbidden, rec.Code, "enforce 坏签名必须拒绝")
		require.Contains(t, rec.Body.String(), constant.AIIdentitySignatureInvalid)
		require.False(t, capture.hasAttribution)
	})
}

func TestAIIdentityAuthDynamicReplayEnforce(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	ctxPayload := `{"root_app_id":"` + boundAppCode(t, tokenID) + `","root_run_id":"run-1"}`
	fn := signedHeaderFn(t, snap, secret, ctxPayload)

	r := newAIMiddlewareRouter(t, tokenID, &aiCapture{})
	first := performAIRequest(r, http.MethodPost, "/v1/chat/completions", fn)
	require.Equal(t, http.StatusNoContent, first.Code)
	second := performAIRequest(r, http.MethodPost, "/v1/chat/completions", fn)
	require.Equal(t, http.StatusForbidden, second.Code, "同一 nonce 重放必须拒绝")
	require.Contains(t, second.Body.String(), constant.AIIdentityReplayDetected)
}

func TestAIIdentityAuthDynamicRedisDown(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	ctxPayload := `{"root_app_id":"` + boundAppCode(t, tokenID) + `","root_run_id":"run-1"}`

	t.Run("audit degrades unverified", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
		common.RDB = nil
		capture := &aiCapture{}
		r := newAIMiddlewareRouter(t, tokenID, capture)
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", signedHeaderFn(t, snap, secret, ctxPayload))
		require.Equal(t, http.StatusNoContent, rec.Code, "audit 存储不可用降级放行")
		require.True(t, capture.hasAttribution)
		assert.False(t, capture.attribution.IdentityVerified, "audit 降级 identity_verified=false")
		assert.False(t, capture.attribution.ClientVerified)
	})

	t.Run("enforce 503", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
		common.RDB = nil
		r := newAIMiddlewareRouter(t, tokenID, &aiCapture{})
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", signedHeaderFn(t, snap, secret, ctxPayload))
		require.Equal(t, http.StatusServiceUnavailable, rec.Code, "enforce 存储不可用 503")
		require.Contains(t, rec.Body.String(), constant.AIReplayStoreUnavailable)
	})
}

func TestAIIdentityAuthDynamicAppNotAllowedEnforce(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeDynamic, constant.AttributionTargetPlatform,
		constant.IdentityAssuranceSignedContext, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	// root_app 未绑定 → APP_NOT_BOUND。
	ctxPayload := `{"root_app_id":"unbound_app","root_run_id":"run-1"}`
	r := newAIMiddlewareRouter(t, tokenID, &aiCapture{})
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", signedHeaderFn(t, snap, secret, ctxPayload))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), constant.AIIdentityAppNotBound)
}

// --- HYBRID / APPLICATION（验收 E / 7.13） ---

func TestAIIdentityAuthHybridApplicationEnforce(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, secret := createAIMiddlewareProfile(t,
		constant.IdentityModeHybrid, constant.AttributionTargetApplication,
		constant.IdentityAssuranceHybridVerified, true, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	ctxPayload := `{"root_app_id":"` + boundAppCode(t, tokenID) + `","root_run_id":"run-1","current_execution_id":"e1","execution_type":"chat"}`

	capture := &aiCapture{}
	r := newAIMiddlewareRouter(t, tokenID, capture)
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", signedHeaderFn(t, snap, secret, ctxPayload))
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, capture.hasAttribution)
	ctx := capture.attribution
	assert.True(t, ctx.ClientVerified, "HYBRID client_verified=true")
	assert.True(t, ctx.IdentityVerified)
	assert.Equal(t, boundAppCode(t, tokenID), ctx.RootAppID)
	assert.Equal(t, constant.IdentityAssuranceHybridVerified, ctx.IdentityAssurance)
}

// --- Profile 缺失 / 停用 / 模式非法（验收 B） ---

func TestAIIdentityAuthProfileMissingEnforce(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID := createAIMiddlewareToken(t) // 有 token 无 profile

	r := newAIMiddlewareRouter(t, tokenID, &aiCapture{})
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), constant.AIIdentityProfileRequired)
}

func TestAIIdentityAuthProfileMissingAuditAllows(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
	setupAIMiddlewareEnv(t)
	tokenID := createAIMiddlewareToken(t)

	r := newAIMiddlewareRouter(t, tokenID, &aiCapture{})
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusNoContent, rec.Code, "audit 无 profile 放行")
}

func TestAIIdentityAuthProfileDisabledEnforce(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	enabled := false
	_, err = service.UpdateIdentityProfile(&service.IdentityProfilePatch{Id: snap.ProfileID, Enabled: &enabled})
	require.NoError(t, err)

	r := newAIMiddlewareRouter(t, tokenID, &aiCapture{})
	rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), constant.AIIdentityProfileDisabled)
}

// D.2：同一 PROFILE_DISABLED 失败按实际处置定 result——audit + c.Next() 降级放行 → UNVERIFIED；
// enforce + abort 实际阻断 → REJECTED。disposition 才是 result 的正式含义，mode 本身不是。
func TestAIIdentityAuthProfileDisabledDispositionResult(t *testing.T) {
	setupAIMiddlewareEnv(t)
	tokenID, _ := createAIMiddlewareProfile(t,
		constant.IdentityModeStatic, constant.AttributionTargetPrincipal,
		constant.IdentityAssuranceCredentialOnly, false, service.IdentityProfilePatch{})
	snap, err := service.GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	enabled := false
	_, err = service.UpdateIdentityProfile(&service.IdentityProfilePatch{Id: snap.ProfileID, Enabled: &enabled})
	require.NoError(t, err)

	t.Run("audit continues → UNVERIFIED", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
		require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
		r := newAIMiddlewareRouter(t, tokenID, &aiCapture{})
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusNoContent, rec.Code, "audit profile disabled 降级放行")
		evs := listIdentityAuditEvents(t)
		require.Len(t, evs, 1)
		assert.Equal(t, constant.IdentityAuditResultUnverified, evs[0].Result)
		assert.Equal(t, constant.ReasonCodeProfileDisabled, evs[0].ReasonCode)
	})
	t.Run("enforce aborts → REJECTED", func(t *testing.T) {
		t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
		require.NoError(t, model.DB.Where("1 = 1").Delete(&model.AIIdentityAuditEvent{}).Error)
		r := newAIMiddlewareRouter(t, tokenID, &aiCapture{})
		rec := performAIRequest(r, http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, http.StatusForbidden, rec.Code, "enforce profile disabled 实际阻断")
		evs := listIdentityAuditEvents(t)
		require.Len(t, evs, 1)
		assert.Equal(t, constant.IdentityAuditResultRejected, evs[0].Result)
		assert.Equal(t, constant.ReasonCodeProfileDisabled, evs[0].ReasonCode)
	})
}

// --- 非消费端点跳过（验收 H） ---

func TestAIIdentityAuthSkipsNonConsumption(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupAIMiddlewareEnv(t)
	// 无 profile 的 token 也通过非消费端点。
	tokenID := createAIMiddlewareToken(t)

	capture := &aiCapture{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "req-skip")
		c.Set("token_id", tokenID)
	})
	r.Use(AIIdentityAuth())
	r.GET("/v1/models", func(c *gin.Context) {
		for _, h := range constant.AIHeaderNames {
			if c.Request.Header.Get(h) != "" {
				capture.remaining = append(capture.remaining, h)
			}
		}
		c.Status(http.StatusNoContent)
	})

	rec := performAIRequest(r, http.MethodGet, "/v1/models", setAllAIHeaders)
	require.Equal(t, http.StatusNoContent, rec.Code, "非消费 GET 端点 enforce 下也应放行")
	require.Empty(t, capture.remaining, "跳过端点仍必须清头")
}
