package channel

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProcessHeaderOverride_ChannelTestSkipsPassthroughRules(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Empty(t, headers)
}

func TestProcessHeaderOverride_ChannelTestSkipsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	_, ok := headers["x-upstream-trace"]
	require.False(t, ok)
}

func TestProcessHeaderOverride_NonTestKeepsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-upstream-trace"])
}

func TestProcessHeaderOverride_RuntimeOverrideIsFinalHeaderMap(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		IsChannelTest:             false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"x-static":  "runtime-value",
			"x-runtime": "runtime-only",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
				"X-Legacy": "legacy-only",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "runtime-value", headers["x-static"])
	require.Equal(t, "runtime-only", headers["x-runtime"])
	_, exists := headers["x-legacy"]
	require.False(t, exists)
}

func TestProcessHeaderOverride_PassthroughSkipsAcceptEncoding(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])

	_, hasAcceptEncoding := headers["accept-encoding"]
	require.False(t, hasAcceptEncoding)
}

func TestProcessHeaderOverride_PassHeadersTemplateSetsRuntimeHeaders(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Originator", "Codex CLI")
	ctx.Request.Header.Set("Session_id", "sess-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]any{
				"operations": []any{
					map[string]any{
						"mode":  "pass_headers",
						"value": []any{"Originator", "Session_id", "X-Codex-Beta-Features"},
					},
				},
			},
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-4.1"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	require.Equal(t, "legacy-value", info.RuntimeHeadersOverride["x-static"])

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "Codex CLI", headers["originator"])
	require.Equal(t, "sess-123", headers["session_id"])
	_, exists = headers["x-codex-beta-features"]
	require.False(t, exists)

	upstreamReq := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	applyHeaderOverrideToRequest(upstreamReq, headers)
	require.Equal(t, "Codex CLI", upstreamReq.Header.Get("Originator"))
	require.Equal(t, "sess-123", upstreamReq.Header.Get("Session_id"))
	require.Empty(t, upstreamReq.Header.Get("X-Codex-Beta-Features"))
}

// TestProcessHeaderOverride_NeverLeaksAIAttributionHeaders 验收 L：
// 六个企业身份协议 Header 一律不得经 wildcard/regex/显式 override/{client_header}
// 透传或注入到上游请求头（纵深防护，大小写不敏感）。
func TestProcessHeaderOverride_NeverLeaksAIAttributionHeaders(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	// 大小写混合的 X-AI-* 入站头（验证大小写不敏感过滤）。
	ctx.Request.Header.Set("X-AI-Context-Version", "v1")
	ctx.Request.Header.Set("x-ai-context", "EnCoded")
	ctx.Request.Header.Set("X-AI-TIMESTAMP", "1700000000")
	ctx.Request.Header.Set("x-ai-NONCE", "abcdefghijklmnopqrstuv")
	ctx.Request.Header.Set("X-AI-Key-Id", "key-1")
	ctx.Request.Header.Set("x-ai-Signature", "sig")
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	aiNames := []string{
		"x-ai-context-version", "x-ai-context", "x-ai-timestamp",
		"x-ai-nonce", "x-ai-key-id", "x-ai-signature",
	}
	assertNone := func(t *testing.T, headers map[string]string) {
		t.Helper()
		for _, n := range aiNames {
			if _, ok := headers[n]; ok {
				t.Fatalf("AI attribution header %q leaked to upstream override map", n)
			}
		}
	}

	t.Run("wildcard", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			IsChannelTest: false,
			ChannelMeta: &relaycommon.ChannelMeta{
				HeadersOverride: map[string]any{"*": ""},
			},
		}
		headers, err := processHeaderOverride(info, ctx)
		require.NoError(t, err)
		assertNone(t, headers)
		require.Equal(t, "trace-123", headers["x-trace-id"])
	})

	t.Run("regex", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			IsChannelTest: false,
			ChannelMeta: &relaycommon.ChannelMeta{
				HeadersOverride: map[string]any{"regex:^x-ai-": ""},
			},
		}
		headers, err := processHeaderOverride(info, ctx)
		require.NoError(t, err)
		assertNone(t, headers)
	})

	t.Run("re-prefix regex", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			IsChannelTest: false,
			ChannelMeta: &relaycommon.ChannelMeta{
				HeadersOverride: map[string]any{"re:AI-": ""},
			},
		}
		headers, err := processHeaderOverride(info, ctx)
		require.NoError(t, err)
		assertNone(t, headers)
	})

	t.Run("explicit override", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			IsChannelTest: false,
			ChannelMeta: &relaycommon.ChannelMeta{
				HeadersOverride: map[string]any{
					"X-AI-Context":     "injected",
					"X-AI-Signature":   "injected",
					"X-Upstream-Trace": "keep-me",
				},
			},
		}
		headers, err := processHeaderOverride(info, ctx)
		require.NoError(t, err)
		assertNone(t, headers)
		require.Equal(t, "keep-me", headers["x-upstream-trace"])
	})

	t.Run("client_header into AI header name", func(t *testing.T) {
		// {client_header:*} 解析出的值即使注入到 X-AI-* 输出头名，也必须在纵深防护被剥离。
		info := &relaycommon.RelayInfo{
			IsChannelTest: false,
			ChannelMeta: &relaycommon.ChannelMeta{
				HeadersOverride: map[string]any{
					"X-AI-Signature": "{client_header:X-Trace-Id}",
				},
			},
		}
		headers, err := processHeaderOverride(info, ctx)
		require.NoError(t, err)
		assertNone(t, headers)
	})
}
