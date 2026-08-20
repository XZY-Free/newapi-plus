package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// aiGovernanceWriteRoutes 是第 1 批 18 个新增 Root 写管理 API 的
// 「METHOD + FullPath → 文档 6.16 语义 action」契约。此表是用户可见审计合约：
// 新增写路由必须在此登记，且不得回退为 generic 兜底。
var aiGovernanceWriteRoutes = []struct {
	method   string
	fullPath string
	action   string
}{
	{"POST", "/api/ai-governance/business-domains", "business_domain.create"},
	{"PUT", "/api/ai-governance/business-domains/:id", "business_domain.update"},
	{"POST", "/api/ai-governance/owner-teams", "owner_team.create"},
	{"PUT", "/api/ai-governance/owner-teams/:id", "owner_team.update"},
	{"POST", "/api/ai-governance/usage-teams", "usage_team.create"},
	{"PUT", "/api/ai-governance/usage-teams/:id", "usage_team.update"},
	{"POST", "/api/ai-governance/principals", "principal.create"},
	{"PUT", "/api/ai-governance/principals/:id", "principal.update"},
	{"POST", "/api/ai-governance/credential-purposes", "credential_purpose.create"},
	{"PUT", "/api/ai-governance/credential-purposes/:id", "credential_purpose.update"},
	{"POST", "/api/ai-governance/applications", "application.create"},
	{"PUT", "/api/ai-governance/applications/:id", "application.update"},
	{"POST", "/api/ai-governance/identity-profiles", "identity_profile.create"},
	{"PUT", "/api/ai-governance/identity-profiles/:id", "identity_profile.update"},
	{"PUT", "/api/ai-governance/identity-profiles/:id/app-bindings", "identity_profile.replace_bindings"},
	{"POST", "/api/ai-governance/identity-profiles/:id/signing-keys/generate", "signing_key.generate"},
	{"POST", "/api/ai-governance/identity-profiles/:id/signing-keys/rotate", "signing_key.rotate"},
	{"POST", "/api/ai-governance/identity-profiles/:id/signing-keys/:key_id/revoke", "signing_key.revoke"},
}

// 门禁 23：全部 18 个写管理 API 必须命中语义化 action，且与文档 6.16 契约一致，
// 绝不允许 generic 兜底；并反向校验审计映射不残留契约之外的治理写路由（防漂移）。
func TestAIGovernanceWriteRoutesAuditContract(t *testing.T) {
	require.Len(t, aiGovernanceWriteRoutes, 18, "第 1 批必须恰好 18 个写管理 API")

	for _, rr := range aiGovernanceWriteRoutes {
		key := rr.method + " " + rr.fullPath
		got, ok := auditRouteActions[key]
		require.Truef(t, ok, "%s 必须在审计映射中登记", key)
		require.NotEqualf(t, "generic", got, "%s 不得回退为 generic 兜底", key)
		require.Equalf(t, rr.action, got, "%s 的 action 必须等于文档 6.16 语义 action", key)
	}

	// 反向：审计映射中所有 ai-governance 写路由都必须恰好对应契约，防新增路由漏登记。
	contractKeys := map[string]bool{}
	for _, rr := range aiGovernanceWriteRoutes {
		contractKeys[rr.method+" "+rr.fullPath] = true
	}
	registered := 0
	for key := range auditRouteActions {
		if strings.Contains(key, "/api/ai-governance/") {
			registered++
			require.Truef(t, contractKeys[key], "审计映射中的治理写路由 %q 必须在契约中登记", key)
		}
	}
	require.Equal(t, len(aiGovernanceWriteRoutes), registered, "治理写路由数量必须与契约一致")
}

// 路由模板到 action 的稳定映射：以与真实路由同构的 Gin engine 复刻这 18 条写路由，
// 逐条请求并断言 auditRouteActions 依据真实 FullPath 解析出的 action 等于契约 action。
// 保护「新增写管理 API 自动留痕且 action 语义化」这一用户可见审计合约。
func TestAIGovernanceWriteRoutesResolveFromFullPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	var mu sync.Mutex
	resolved := map[string]string{}
	handler := func(c *gin.Context) {
		key := c.Request.Method + " " + c.FullPath()
		mu.Lock()
		resolved[key] = auditRouteActions[key]
		mu.Unlock()
		c.Status(http.StatusOK)
	}
	for _, rr := range aiGovernanceWriteRoutes {
		r.Handle(rr.method, rr.fullPath, handler)
	}

	for _, rr := range aiGovernanceWriteRoutes {
		url := pathParamsReplaced(rr.fullPath)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(rr.method, url, nil))
		require.Equal(t, http.StatusOK, rec.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, rr := range aiGovernanceWriteRoutes {
		got := resolved[rr.method+" "+rr.fullPath]
		require.Equalf(t, rr.action, got, "真实 FullPath 解析的 action 必须等于契约 action")
	}
}

// 审计参数只记录路径参数；断言 18 条写路由的路径参数集合只有良性 id/key_id，
// 绝不包含 Token Key 或 Signing Secret（它们不会作为路径参数出现，且 key_id
// 仅是标识，不含密钥明文）。
func TestAIGovernanceWriteRoutesNoSecretInAuditParams(t *testing.T) {
	paramKeys := map[string]bool{}
	for _, rr := range aiGovernanceWriteRoutes {
		for _, seg := range strings.Split(rr.fullPath, "/") {
			if strings.HasPrefix(seg, ":") {
				paramKeys[strings.TrimPrefix(seg, ":")] = true
			}
		}
	}
	for k := range paramKeys {
		require.Contains(t, []string{"id", "key_id"}, k,
			"写路由路径参数只能为良性 id/key_id，不得携带 secret/token key")
	}
}

// pathParamsReplaced 将路由模板中的路径参数替换为示例值以构造可请求的 URL。
func pathParamsReplaced(fullPath string) string {
	url := fullPath
	url = strings.ReplaceAll(url, ":key_id", "abc123")
	url = strings.ReplaceAll(url, ":id", "1")
	return url
}
