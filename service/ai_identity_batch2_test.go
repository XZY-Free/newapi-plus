package service

import (
	"context"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- 运行时模式（验收 B） ---

func TestGetAttributionMode(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, "")
	require.Equal(t, constant.AttributionModeDisabled, GetAttributionMode(), "未设置默认 disabled")

	t.Setenv(constant.AttributionModeEnv, "audit")
	require.Equal(t, constant.AttributionModeAudit, GetAttributionMode())

	t.Setenv(constant.AttributionModeEnv, "enforce")
	require.Equal(t, constant.AttributionModeEnforce, GetAttributionMode())

	t.Setenv(constant.AttributionModeEnv, "bogus")
	require.Equal(t, "", GetAttributionMode(), "非法值返回空串，调用方必须 fail-closed")
}

// --- X-AI-Context 解码（验收 C：严格 Schema） ---

func b64url(plain string) string { return base64.RawURLEncoding.EncodeToString([]byte(plain)) }

func TestDecodeSignedContextStrict(t *testing.T) {
	valid := `{"root_run_id":"run-1","root_app_id":"hr_assistant","current_execution_id":"e1","execution_type":"chat","execution_depth":2}`
	ctx, err := DecodeSignedContext(b64url(valid))
	require.NoError(t, err)
	require.Equal(t, "run-1", ctx.RootRunID)
	require.Equal(t, "hr_assistant", ctx.RootAppID)
	require.Equal(t, "e1", ctx.CurrentExecutionID)
	require.Equal(t, 2, *ctx.ExecutionDepth)

	// 未知字段拒绝。
	_, err = DecodeSignedContext(b64url(`{"root_run_id":"r","unknown":"x"}`))
	require.Error(t, err, "未知字段必须拒绝")

	// 尾随 JSON 拒绝。
	_, err = DecodeSignedContext(b64url(`{"root_run_id":"r"} {"x":1}`))
	require.Error(t, err, "尾随 JSON 必须拒绝")

	// 非法 UTF-8 拒绝（0xFF 不是合法 UTF-8）。
	badUTF8 := b64url(`{"root_run_id":"r"}`) + base64.RawURLEncoding.EncodeToString([]byte{0xff})
	_, err = DecodeSignedContext(badUTF8)
	require.Error(t, err, "非法 UTF-8 必须拒绝")

	// 非 Base64URL 字符拒绝。
	_, err = DecodeSignedContext("!!!not-base64!!!")
	require.Error(t, err, "非 Base64URL 字符必须拒绝")

	// 空串拒绝。
	_, err = DecodeSignedContext("")
	require.Error(t, err, "空 Context 必须拒绝")

	// 超过 8192 编码长度拒绝。
	big := strings.Repeat("A", constant.AttributionContextMaxEncoded+1)
	_, err = DecodeSignedContext(big)
	require.Error(t, err, "编码长度超过 8192 必须拒绝")
}

func TestDecodeSignedContextDecodedSizeLimit(t *testing.T) {
	// 解码后超过 6144 字节必须拒绝。
	payload := `{"root_run_id":"` + strings.Repeat("r", 6200) + `"}`
	_, err := DecodeSignedContext(b64url(payload))
	require.Error(t, err, "解码后超过 6144 字节必须拒绝")
}

// --- Context 字段依赖（验收 C / 7.3） ---

func TestValidateExecutionContext(t *testing.T) {
	intPtr := func(i int) *int { return &i }
	t.Run("root_run_id required", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: ""}, constant.IdentityModeDynamic)
		require.Error(t, err)
	})
	t.Run("dynamic requires root_app_id", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: "r"}, constant.IdentityModeDynamic)
		require.Error(t, err, "DYNAMIC root_app_id 必填")
	})
	t.Run("hybrid root_app optional", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: "r"}, constant.IdentityModeHybrid)
		require.NoError(t, err)
	})
	t.Run("current exec requires current_execution_id", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: "r", RootAppID: "a", ExecutionDepth: intPtr(1)}, constant.IdentityModeDynamic)
		require.Error(t, err, "有任意当前执行字段时必须提供 current_execution_id")
	})
	t.Run("execution_type requires current_execution_id", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: "r", RootAppID: "a", ExecutionType: "chat"}, constant.IdentityModeDynamic)
		require.Error(t, err, "execution_type 存在时 current_execution_id 必填")
	})
	t.Run("current_execution_id requires execution_type", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: "r", RootAppID: "a", CurrentExecutionID: "e1"}, constant.IdentityModeDynamic)
		require.Error(t, err, "current_execution_id 存在时 execution_type 必填")
	})
	t.Run("paired current_execution_id and execution_type accepted", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: "r", RootAppID: "a", CurrentExecutionID: "e1", ExecutionType: "chat"}, constant.IdentityModeDynamic)
		require.NoError(t, err, "current_execution_id 与 execution_type 成对出现必须通过")
	})
	t.Run("depth range", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: "r", RootAppID: "a", ExecutionDepth: intPtr(65)}, constant.IdentityModeDynamic)
		require.Error(t, err, "execution_depth 超过 64 必须拒绝")
		err = ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: "r", RootAppID: "a", ExecutionDepth: intPtr(-1)}, constant.IdentityModeDynamic)
		require.Error(t, err, "execution_depth 为负必须拒绝")
	})
	t.Run("root_run_id length", func(t *testing.T) {
		err := ValidateExecutionContext(&types.SignedExecutionContext{RootRunID: strings.Repeat("r", 129), RootAppID: "a"}, constant.IdentityModeDynamic)
		require.Error(t, err)
	})
}

// --- Canonical / HMAC（验收 C / 7.4） ---

func TestBuildCanonicalString(t *testing.T) {
	got := BuildCanonicalString("post", "/kling/v1/videos/text2video", "1700000000", "abcdefghijklmnopqrstuv", "key-1", "ENCCTX")
	want := "v1\nPOST\n/kling/v1/videos/text2video\n1700000000\nabcdefghijklmnopqrstuv\nkey-1\nENCCTX"
	require.Equal(t, want, got, "7 行 LF 分隔、末行后无 LF、method 大写、path 无 query")
	require.False(t, strings.HasSuffix(got, "\n"), "末行后不得有 LF")
}

func TestHMACSHA256HexAndConstantTime(t *testing.T) {
	// 已知向量：secret=key，message=canonical。
	canonical := "v1\nPOST\n/path\n1\nnonce\nkey\nctx"
	sig := HMACSHA256Hex([]byte("key"), canonical)
	require.Len(t, sig, 64)
	require.Equal(t, strings.ToLower(sig), sig, "必须小写")
	require.True(t, ConstantTimeHexEqual(sig, sig))
	require.False(t, ConstantTimeHexEqual(sig, strings.Repeat("0", 64)))
	// 长度不同必须不相等。
	require.False(t, ConstantTimeHexEqual(sig, "abc"))
}

// --- Nonce / Timestamp（验收 C / 7.5 / 7.6） ---

func TestValidateNonceFormat(t *testing.T) {
	require.False(t, ValidateNonceFormat("short"))                                                // < 22
	require.False(t, ValidateNonceFormat(strings.Repeat("a", constant.AttributionNonceMaxLen+1))) // > 64
	require.False(t, ValidateNonceFormat(strings.Repeat("a+b", 20)))                              // 非法字符 +
	require.True(t, ValidateNonceFormat("abcdefghijklmnopqrstuv_wxyz0123456789-_A"))
}

func TestValidateTimestampSkew(t *testing.T) {
	now := common.GetTimestamp()
	require.True(t, ValidateTimestamp(nowString(now), now))
	require.True(t, ValidateTimestamp(nowString(now+300), now))
	require.False(t, ValidateTimestamp(nowString(now+301), now), "超出偏差必须拒绝")
	require.False(t, ValidateTimestamp("not-a-number", now))
}

func nowString(ts int64) string { return strconv.FormatInt(ts, 10) }

// --- 集中分类器 IsAttributionRequired（验收 H / 7.16） ---

func TestIsAttributionRequired(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		// 生成/消费 POST：必须归因。
		{"chat", http.MethodPost, "/v1/chat/completions", true},
		{"messages", http.MethodPost, "/v1/messages", true},
		{"responses", http.MethodPost, "/v1/responses", true},
		{"images", http.MethodPost, "/v1/images/generations", true},
		{"audio", http.MethodPost, "/v1/audio/speech", true},
		{"embeddings", http.MethodPost, "/v1/embeddings", true},
		{"rerank", http.MethodPost, "/v1/rerank", true},
		{"moderation", http.MethodPost, "/v1/moderations", true},
		{"gemini generate", http.MethodPost, "/v1beta/models/gemini-pro:generateContent", true},
		{"suno submit", http.MethodPost, "/suno/submit/music", true},
		{"mj submit", http.MethodPost, "/mj/submit/imagine", true},
		{"video generation", http.MethodPost, "/v1/video/generations", true},
		{"video openai", http.MethodPost, "/v1/videos", true},
		{"video remix", http.MethodPost, "/v1/videos/vid_1/remix", true},
		{"kling submit", http.MethodPost, "/kling/v1/videos/text2video", true},
		{"jimeng submit", http.MethodPost, "/jimeng/", true},
		{"realtime ws", http.MethodGet, "/v1/realtime", true},
		// 只读/查询/下载/fetch：不要求新消费验证。
		{"models get", http.MethodGet, "/v1/models", false},
		{"models get gemini", http.MethodGet, "/v1beta/models", false},
		{"model retrieve", http.MethodGet, "/v1/models/gpt-4o", false},
		{"files get", http.MethodGet, "/v1/files", false},
		{"file content", http.MethodGet, "/v1/files/f1/content", false},
		{"suno fetch", http.MethodGet, "/suno/fetch/s1", false},
		{"mj task fetch", http.MethodGet, "/mj/task/t1/fetch", false},
		{"mj image", http.MethodGet, "/mj/image/i1", false},
		{"video status", http.MethodGet, "/v1/video/generations/t1", false},
		{"video content", http.MethodGet, "/v1/videos/t1/content", false},
		{"kling fetch", http.MethodGet, "/kling/v1/videos/text2video/t1", false},
		{"jimeng getresult", http.MethodGet, "/v1/video/generations/t1", false},
		{"suno fetch post", http.MethodPost, "/suno/fetch", false},
		{"mj list condition", http.MethodPost, "/mj/task/list-by-condition", false},
		{"mj mode list condition", http.MethodPost, "/audit/mj/task/list-by-condition", false},
		{"mj upload images", http.MethodPost, "/mj/submit/upload-discord-images", false},
		{"mj mode upload images", http.MethodPost, "/enforce/mj/submit/upload-discord-images", false},
		{"mj root upload", http.MethodPost, "/mj", false},
		{"mj mode root upload", http.MethodPost, "/enforce/mj", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, IsAttributionRequired(tc.method, tc.path))
		})
	}
}

// --- 防重放 ClaimNonce（验收 D / 7.6） ---

func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	prevRDB := common.RDB
	prevEnabled := common.RedisEnabled
	srv := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	common.RDB = rdb
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = rdb.Close()
		common.RDB = prevRDB
		common.RedisEnabled = prevEnabled
	})
	return srv, rdb
}

func TestClaimNonceReplayAndStore(t *testing.T) {
	_, rdb := setupMiniRedis(t)
	nonce := "abcdefghijklmnopqrstuv"
	require.NoError(t, ClaimNonce(context.Background(), 101, nonce))
	require.ErrorIs(t, ClaimNonce(context.Background(), 101, nonce), ErrNonceReplay, "同一 nonce 再次使用必须判为重放")
	// 不同 profile 独立。
	require.NoError(t, ClaimNonce(context.Background(), 102, nonce))

	// Redis 不可用 → 存储不可用。
	_ = rdb.Close()
	require.ErrorIs(t, ClaimNonce(context.Background(), 101, "differentnonce12345"), ErrNonceStoreUnavailable)
}

func TestValidateTimestampBoundaries(t *testing.T) {
	now := int64(1700000000)
	t.Run("blank rejected", func(t *testing.T) {
		require.False(t, ValidateTimestamp("", now), "空串必须拒绝")
		require.False(t, ValidateTimestamp("   ", now), "空白串必须拒绝")
	})
	t.Run("min int64 rejected without overflow", func(t *testing.T) {
		require.False(t, ValidateTimestamp("-9223372036854775808", now), "MinInt64 必须拒绝，不得因溢出误放行")
	})
	t.Run("max int64 rejected without overflow", func(t *testing.T) {
		require.False(t, ValidateTimestamp("9223372036854775807", now), "MaxInt64 必须拒绝")
	})
	t.Run("boundary skew accepted", func(t *testing.T) {
		require.True(t, ValidateTimestamp(strconv.FormatInt(now+300, 10), now), "上限内必须接受")
	})
	t.Run("non numeric rejected", func(t *testing.T) {
		require.False(t, ValidateTimestamp("abc", now))
	})
}

// --- Snapshot 缓存 TTL 契约（文档 7.15）+ Clone 不回归 ---

func TestSnapshotCacheTTLContract(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	require.EqualValues(t, 10, snapshotCacheTTLSeconds(), "ENFORCE 快照缓存 TTL 必须为 10 秒")

	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeAudit)
	require.EqualValues(t, 30, snapshotCacheTTLSeconds(), "AUDIT 快照缓存 TTL 必须为 30 秒")

	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeDisabled)
	require.EqualValues(t, 30, snapshotCacheTTLSeconds(), "disabled 快照缓存 TTL 必须为 30 秒")
}

func TestIdentitySnapshotCloneNoMutationRegression(t *testing.T) {
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeStatic,
		AttributionTargetType: constant.AttributionTargetApplication,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		Environment:           "prod",
	}, []int{f.AppID})
	require.NoError(t, err)

	snap, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.NotNil(t, snap)

	// 修改返回快照不得污染缓存。
	snap.ProfileID = 999999
	snap.RateLimit.MaxRequests = 1
	snap.CallerID = "mutated"
	if len(snap.Applications) > 0 {
		snap.Applications[0].AppCode = "mutated"
	}

	again, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.NotNil(t, again)
	require.Equal(t, p.Id, again.ProfileID, "clone 必须隔离修改")
	require.Equal(t, "prod", again.Environment)
}

// --- Profile 凭证限流（验收 I / 6.15.1） ---

func TestAllowProfileRateLimitWindow(t *testing.T) {
	_, _ = setupMiniRedis(t)
	window, max := 60, 2
	allowed, err := AllowProfileRateLimit(context.Background(), 101, window, max, "req-1")
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = AllowProfileRateLimit(context.Background(), 101, window, max, "req-2")
	require.NoError(t, err)
	require.True(t, allowed)
	// 达到上限后下一请求 429（allowed=false）。
	allowed, err = AllowProfileRateLimit(context.Background(), 101, window, max, "req-3")
	require.NoError(t, err)
	require.False(t, allowed, "同 Profile 达到阈值后必须拒绝")
	// 不同 profile 独立计数。
	allowed, err = AllowProfileRateLimit(context.Background(), 102, window, max, "req-1")
	require.NoError(t, err)
	require.True(t, allowed, "不同 Profile 计数必须独立")
}

func TestAllowProfileRateLimitStoreUnavailable(t *testing.T) {
	setupMiniRedis(t)
	// 关闭 RDB 模拟不可用。
	common.RDB = nil
	allowed, err := AllowProfileRateLimit(context.Background(), 101, 60, 10, "req-1")
	require.Error(t, err, "Redis 不可用必须返回错误（audit 放行 / enforce 503）")
	require.False(t, allowed)
}

// --- VerifyStrongIdentity 端到端（验收 C/D/E 强身份） ---

// setupVerifiedDynamic 构造一个已启用、已绑定应用、含 ACTIVE 签名密钥的 DYNAMIC 快照，
// 并返回原始签名密钥。
func setupVerifiedDynamic(t *testing.T) (*types.IdentitySnapshot, []byte) {
	t.Helper()
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	t.Setenv(constant.AIAttributionMasterKeyEnv, masterKey32(0x42))

	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeDynamic,
		AttributionTargetType: constant.AttributionTargetPlatform,
		IdentityAssurance:     constant.IdentityAssuranceSignedContext,
		CallerId:              "workflow-prod",
		Environment:           "prod",
	}, []int{f.AppID})
	require.NoError(t, err)
	_, display, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)
	secret, err := base64.RawURLEncoding.DecodeString(display)
	require.NoError(t, err)

	enabled := true
	_, err = UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, Enabled: &enabled})
	require.NoError(t, err)

	snap, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.True(t, snap.Enabled)
	require.True(t, snap.HasActiveSigningKey)
	require.Len(t, snap.Applications, 1)
	return snap, secret
}

func TestVerifyStrongIdentitySuccess(t *testing.T) {
	srv, _ := setupMiniRedis(t)
	snap, secret := setupVerifiedDynamic(t)
	keyID := snap.SigningKeys[0].KeyId

	ctxPayload := `{"root_app_id":"hr_assistant","root_run_id":"run-9","current_execution_id":"e1","execution_type":"chat"}`
	encoded := b64url(ctxPayload)
	timestamp := nowString(common.GetTimestamp())
	nonce := "abcdefghijklmnopqrstuv"
	canonical := BuildCanonicalString("POST", "/v1/chat/completions", timestamp, nonce, keyID, encoded)
	signature := HMACSHA256Hex(secret, canonical)

	hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: encoded, Timestamp: timestamp, Nonce: nonce, KeyId: keyID, Signature: signature}
	tc, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
	require.Nil(t, failure)
	require.NotNil(t, tc)
	assert.True(t, tc.CredentialVerified)
	assert.True(t, tc.ClientVerified)
	assert.True(t, tc.IdentityVerified)
	assert.Equal(t, constant.IdentityAssuranceSignedContext, tc.IdentityAssurance)
	assert.Equal(t, "hr_assistant", tc.RootAppID)
	assert.Equal(t, "run-9", tc.RootRunID)
	assert.Equal(t, "workflow-prod", tc.CallerID)

	// 同一 nonce 重放 → REPLAY_DETECTED。
	tc, failure = VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
	require.NotNil(t, failure)
	assert.Equal(t, string(constant.AIIdentityReplayDetected), failure.Code)

	// nonce key 必须写入 Redis。
	require.True(t, srv.Exists(constant.IdentityNonceKeyPrefix+strconv.Itoa(snap.ProfileID)+":"+nonce))
}

func TestVerifyStrongIdentityFailures(t *testing.T) {
	setupMiniRedis(t)
	snap, secret := setupVerifiedDynamic(t)
	keyID := snap.SigningKeys[0].KeyId
	ctxPayload := `{"root_app_id":"hr_assistant","root_run_id":"run-9"}`
	encoded := b64url(ctxPayload)
	timestamp := nowString(common.GetTimestamp())
	nonce := "abcdefghijklmnopqrstuv"
	canonical := BuildCanonicalString("POST", "/v1/chat/completions", timestamp, nonce, keyID, encoded)
	signature := HMACSHA256Hex(secret, canonical)

	t.Run("bad signature", func(t *testing.T) {
		hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: encoded, Timestamp: timestamp, Nonce: nonce, KeyId: keyID, Signature: strings.Repeat("0", 64)}
		_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
		require.NotNil(t, failure)
		assert.Equal(t, string(constant.AIIdentitySignatureInvalid), failure.Code)
	})
	t.Run("bad timestamp", func(t *testing.T) {
		hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: encoded, Timestamp: nowString(common.GetTimestamp() + 5000), Nonce: "bbbbbbbbbbbbbbbbbbbbbb", KeyId: keyID, Signature: signature}
		_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
		require.NotNil(t, failure)
		assert.Equal(t, string(constant.AIIdentityTimestampInvalid), failure.Code)
	})
	t.Run("context required", func(t *testing.T) {
		hdr := &AIIdentityHeaders{ContextVersion: "v1", Timestamp: timestamp, Nonce: "cccccccccccccccccccccc", KeyId: keyID, Signature: signature}
		_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
		require.NotNil(t, failure)
		assert.Equal(t, string(constant.AIIdentityContextRequired), failure.Code)
	})
	t.Run("app not bound", func(t *testing.T) {
		ctx := `{"root_app_id":"unbound_app","root_run_id":"run-x"}`
		enc := b64url(ctx)
		canon := BuildCanonicalString("POST", "/v1/chat/completions", timestamp, "dddddddddddddddddddddd", keyID, enc)
		sig := HMACSHA256Hex(secret, canon)
		hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: enc, Timestamp: timestamp, Nonce: "dddddddddddddddddddddd", KeyId: keyID, Signature: sig}
		_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
		require.NotNil(t, failure)
		assert.Equal(t, string(constant.AIIdentityAppNotBound), failure.Code)
	})
	t.Run("nonce invalid format", func(t *testing.T) {
		hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: encoded, Timestamp: timestamp, Nonce: "short", KeyId: keyID, Signature: signature}
		_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
		require.NotNil(t, failure)
		assert.Equal(t, string(constant.AIIdentityNonceInvalid), failure.Code)
	})
	t.Run("signature not strict lower hex uppercase", func(t *testing.T) {
		hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: encoded, Timestamp: timestamp, Nonce: "eeeeeeeeeeeeeeeeeeeeee", KeyId: keyID, Signature: strings.ToUpper(signature)}
		_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
		require.NotNil(t, failure)
		assert.Equal(t, string(constant.AIIdentitySignatureInvalid), failure.Code, "大写 hex 必须视为 SIGNATURE_INVALID")
	})
	t.Run("signature not strict hex wrong length", func(t *testing.T) {
		hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: encoded, Timestamp: timestamp, Nonce: "ffffffffffffffffffffff", KeyId: keyID, Signature: "abc"}
		_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
		require.NotNil(t, failure)
		assert.Equal(t, string(constant.AIIdentitySignatureInvalid), failure.Code, "非 64 位必须视为 SIGNATURE_INVALID")
	})
	t.Run("context too large encoded", func(t *testing.T) {
		hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: strings.Repeat("A", constant.AttributionContextMaxEncoded+1), Timestamp: timestamp, Nonce: "aaaaaaaaaaaaaaaaaaaaaa", KeyId: keyID, Signature: signature}
		_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
		require.NotNil(t, failure)
		assert.Equal(t, string(constant.AIIdentityContextTooLarge), failure.Code)
	})
}

func TestVerifyStrongIdentityHybridAppMismatch(t *testing.T) {
	setupMiniRedis(t)
	setupAIGovernanceService(t)
	f := setupAIMasterData(t)
	tokenID := createAIToken(t)
	t.Setenv(constant.AIAttributionMasterKeyEnv, masterKey32(0x51))

	p, err := CreateIdentityProfile(&model.AIIdentityProfile{
		TokenId: tokenID, IdentityMode: constant.IdentityModeHybrid,
		AttributionTargetType: constant.AttributionTargetApplication,
		IdentityAssurance:     constant.IdentityAssuranceHybridVerified,
		CallerId:              "hybrid-prod",
		Environment:           "prod",
	}, []int{f.AppID})
	require.NoError(t, err)
	_, display, err := GenerateSigningKey(p.Id)
	require.NoError(t, err)
	secret, err := base64.RawURLEncoding.DecodeString(display)
	require.NoError(t, err)
	enabled := true
	_, err = UpdateIdentityProfile(&IdentityProfilePatch{Id: p.Id, Enabled: &enabled})
	require.NoError(t, err)

	snap, err := GetIdentitySnapshotByTokenID(tokenID)
	require.NoError(t, err)
	require.Len(t, snap.Applications, 1)
	keyID := snap.SigningKeys[0].KeyId
	timestamp := nowString(common.GetTimestamp())

	// root_app_id 与固定 App 不一致 → HYBRID_APP_MISMATCH。
	ctx := `{"root_app_id":"wrong_app","root_run_id":"run-h"}`
	enc := b64url(ctx)
	canon := BuildCanonicalString("POST", "/v1/chat/completions", timestamp, "gggggggggggggggggggggg", keyID, enc)
	sig := HMACSHA256Hex(secret, canon)
	hdr := &AIIdentityHeaders{ContextVersion: "v1", Context: enc, Timestamp: timestamp, Nonce: "gggggggggggggggggggggg", KeyId: keyID, Signature: sig}
	_, failure := VerifyStrongIdentity(snap, hdr, "POST", "/v1/chat/completions", common.GetTimestamp())
	require.NotNil(t, failure)
	assert.Equal(t, string(constant.AIIdentityHybridAppMismatch), failure.Code)
	assert.Equal(t, constant.ReasonCodeHybridAppMismatch, failure.Reason)
}
