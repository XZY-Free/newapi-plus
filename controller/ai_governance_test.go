package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupAIGovernanceControllerTestDB 迁移治理表 + tokens 表，并清空，保证测试独立。
func setupAIGovernanceControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(append(model.AIGovernanceModels(), &model.Token{})...))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createAIGovernanceControllerToken(t *testing.T, key string) int {
	t.Helper()
	tk := model.Token{
		Key:            key,
		UserId:         1,
		Status:         1,
		Name:           "governance-token",
		CreatedTime:    common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&tk).Error)
	return tk.Id
}

func doAIGovernanceRequest(t *testing.T, handler gin.HandlerFunc, method, path, body string, params map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	// 带 id 时强制走 /g/:id，URL 使用真实 id 由 gin 填充路径参数。
	router.Handle(method, "/g", handler)
	router.Handle(method, "/g/:id", handler)
	routePath := "/g"
	if v, ok := params["id"]; ok {
		routePath = "/g/" + v
	}
	// 真实保留传入 path 的 query string（如 /g?keyword=fin），拼接到最终请求路径，
	// 否则过滤参数会被静默丢弃，导致「假通过」的覆盖。
	if idx := strings.Index(path, "?"); idx >= 0 {
		routePath += path[idx:]
	}
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, routePath, reader)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeAIGovernanceResponse(t *testing.T, recorder *httptest.ResponseRecorder) (bool, string, json.RawMessage) {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload.Success, payload.Message, payload.Data
}

// 门禁 1：迁移 + 表名（controller 层复用真实迁移路径 AIGovernanceModels）。
func TestAIGovernanceControllerMasterDataFlow(t *testing.T) {
	setupAIGovernanceControllerTestDB(t)

	// 创建 Domain。
	rec := doAIGovernanceRequest(t, CreateBusinessDomain, http.MethodPost, "/g", `{"domain_code":"finance","domain_name":"财务"}`, nil)
	ok, _, _ := decodeAIGovernanceResponse(t, rec)
	require.True(t, ok)

	// 创建 Owner/Usage Team。
	doAIGovernanceRequest(t, CreateOwnerTeam, http.MethodPost, "/g", `{"team_code":"ai_application","team_name":"AI应用组"}`, nil)
	doAIGovernanceRequest(t, CreateUsageTeam, http.MethodPost, "/g", `{"team_code":"finance_digital","team_name":"财务数字化组"}`, nil)

	// 查询 Domain 列表确认分页/过滤可用。
	rec = doAIGovernanceRequest(t, GetBusinessDomains, http.MethodGet, "/g?keyword=fin", "", nil)
	ok, _, data := decodeAIGovernanceResponse(t, rec)
	require.True(t, ok)
	var page struct {
		Items []model.AIBusinessDomain `json:"items"`
		Total int64                    `json:"total"`
	}
	require.NoError(t, common.Unmarshal(data, &page))
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, "finance", page.Items[0].DomainCode)
}

// 门禁 13/22：Profile 详情绝不返回 Token Key；DYNAMIC 无 ACTIVE 密钥不能启用。
func TestAIGovernanceProfileDetailNoTokenKey(t *testing.T) {
	db := setupAIGovernanceControllerTestDB(t)
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))

	// 创建主数据。
	_, _, dd := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateBusinessDomain, http.MethodPost, "/g", `{"domain_code":"finance","domain_name":"财务"}`, nil))
	var domain model.AIBusinessDomain
	require.NoError(t, common.Unmarshal(dd, &domain))
	_, _, dot := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateOwnerTeam, http.MethodPost, "/g", `{"team_code":"ai_application","team_name":"AI应用组"}`, nil))
	var ownerTeam model.AIOwnerTeam
	require.NoError(t, common.Unmarshal(dot, &ownerTeam))
	_, _, dt := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateUsageTeam, http.MethodPost, "/g", `{"team_code":"finance_digital","team_name":"财务数字化组"}`, nil))
	var usageTeam model.AIUsageTeam
	require.NoError(t, common.Unmarshal(dt, &usageTeam))
	_, _, dp := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreatePrincipal, http.MethodPost, "/g", fmt.Sprintf(`{"principal_code":"zhangsan","principal_name":"张三","business_domain_id":%d,"usage_team_id":%d}`, domain.Id, usageTeam.Id), nil))
	var principal model.AIPrincipal
	require.NoError(t, common.Unmarshal(dp, &principal))
	_, _, dpu := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateCredentialPurpose, http.MethodPost, "/g", `{"purpose_code":"workbuddy","purpose_name":"WorkBuddy","purpose_type":"DESKTOP_CLIENT"}`, nil))
	var purpose model.AICredentialPurpose
	require.NoError(t, common.Unmarshal(dpu, &purpose))
	_, _, da := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateApplication, http.MethodPost, "/g", fmt.Sprintf(`{"app_code":"hr_assistant","app_name":"人力助手","business_domain_id":%d,"owner_team_id":%d}`, domain.Id, ownerTeam.Id), nil))
	var app model.AIApplication
	require.NoError(t, common.Unmarshal(da, &app))

	tokenKey := "sk-controller-" + common.GetRandomString(20)
	tokenID := createAIGovernanceControllerToken(t, tokenKey)

	// 创建 DYNAMIC/PLATFORM Profile（disabled）。
	_, _, dpf := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateIdentityProfile, http.MethodPost, "/g", fmt.Sprintf(`{"token_id":%d,"identity_mode":"DYNAMIC","attribution_target_type":"PLATFORM","identity_assurance":"SIGNED_CONTEXT","caller_id":"workflow-prod","app_ids":[%d]}`, tokenID, app.Id), nil))
	var profile model.AIIdentityProfile
	require.NoError(t, common.Unmarshal(dpf, &profile))

	// 无 ACTIVE 密钥不能启用 DYNAMIC/PLATFORM：真实 API 为 PUT，必须返回失败且 DB 保持 disabled。
	rec := doAIGovernanceRequest(t, UpdateIdentityProfile, http.MethodPut, "/g", `{"enabled":true}`, map[string]string{"id": fmt.Sprintf("%d", profile.Id)})
	ok, msg, _ := decodeAIGovernanceResponse(t, rec)
	require.False(t, ok, "DYNAMIC/PLATFORM 无 ACTIVE 密钥不得启用: %s", msg)
	var afterDisable model.AIIdentityProfile
	require.NoError(t, model.DB.First(&afterDisable, profile.Id).Error)
	require.False(t, afterDisable.Enabled, "失败的启用请求不得改变 DB")

	// 生成签名密钥。
	_, _, dgen := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, GenerateSigningKey, http.MethodPost, "/g", "", map[string]string{"id": fmt.Sprintf("%d", profile.Id)}))
	var genResp struct {
		Key    map[string]interface{} `json:"key"`
		Secret string                 `json:"secret"`
	}
	require.NoError(t, common.Unmarshal(dgen, &genResp))
	require.NotEmpty(t, genResp.Secret, "generate 只本次返回明文")

	// 启用成功。
	rec = doAIGovernanceRequest(t, UpdateIdentityProfile, http.MethodPut, "/g", `{"enabled":true}`, map[string]string{"id": fmt.Sprintf("%d", profile.Id)})
	ok, msg, _ = decodeAIGovernanceResponse(t, rec)
	require.True(t, ok, msg)

	// Profile 详情：绝不返回 Token Key，返回风险姿态与空限流摘要。
	rec = doAIGovernanceRequest(t, GetIdentityProfile, http.MethodGet, "/g", "", map[string]string{"id": fmt.Sprintf("%d", profile.Id)})
	ok, _, data := decodeAIGovernanceResponse(t, rec)
	require.True(t, ok)
	body := string(data)
	require.NotContains(t, body, tokenKey, "Profile 详情不得返回 Token Key")
	require.NotContains(t, body, "sk-controller", "Profile 详情不得返回 Token Key 前缀")
	require.Contains(t, body, "token_id")
	require.Contains(t, body, "risk")

	// Signing Key 列表：绝不含密文/明文。
	rec = doAIGovernanceRequest(t, GetSigningKeys, http.MethodGet, "/g", "", map[string]string{"id": fmt.Sprintf("%d", profile.Id)})
	ok, _, data = decodeAIGovernanceResponse(t, rec)
	require.True(t, ok)
	body = string(data)
	require.NotContains(t, body, genResp.Secret, "密钥列表不得返回明文")
	require.NotContains(t, body, "secret_ciphertext", "密钥列表不得暴露密文字段")

	_ = db
}

// 门禁 24：弱身份且无任何保护 → 详情中 risk_level 必须为 HIGH。
func TestAIGovernanceProfileDetailHighRiskBottomLine(t *testing.T) {
	setupAIGovernanceControllerTestDB(t)

	// 弱身份 CREDENTIAL_ONLY + 无 IP/Model/Quota/Expiry 限制 → HIGH。
	tokenKey := "sk-weak-" + common.GetRandomString(20)
	tokenID := createAIGovernanceControllerToken(t, tokenKey)

	// 先构造一个可启用的 STATIC/PRINCIPAL Profile。
	_, _, dd := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateBusinessDomain, http.MethodPost, "/g", `{"domain_code":"finance","domain_name":"财务"}`, nil))
	var domain model.AIBusinessDomain
	require.NoError(t, common.Unmarshal(dd, &domain))
	_, _, dt := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateUsageTeam, http.MethodPost, "/g", `{"team_code":"finance_digital","team_name":"财务数字化组"}`, nil))
	var usageTeam model.AIUsageTeam
	require.NoError(t, common.Unmarshal(dt, &usageTeam))
	_, _, dp := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreatePrincipal, http.MethodPost, "/g", fmt.Sprintf(`{"principal_code":"zhangsan","principal_name":"张三","business_domain_id":%d,"usage_team_id":%d}`, domain.Id, usageTeam.Id), nil))
	var principal model.AIPrincipal
	require.NoError(t, common.Unmarshal(dp, &principal))
	_, _, dpu := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateCredentialPurpose, http.MethodPost, "/g", `{"purpose_code":"workbuddy","purpose_name":"WorkBuddy","purpose_type":"DESKTOP_CLIENT"}`, nil))
	var purpose model.AICredentialPurpose
	require.NoError(t, common.Unmarshal(dpu, &purpose))

	_, _, dpf := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateIdentityProfile, http.MethodPost, "/g", fmt.Sprintf(`{"token_id":%d,"identity_mode":"STATIC","attribution_target_type":"PRINCIPAL","identity_assurance":"CREDENTIAL_ONLY","principal_id":%d,"credential_purpose_id":%d,"enabled":true}`, tokenID, principal.Id, purpose.Id), nil))
	var profile model.AIIdentityProfile
	require.NoError(t, common.Unmarshal(dpf, &profile))

	rec := doAIGovernanceRequest(t, GetIdentityProfile, http.MethodGet, "/g", "", map[string]string{"id": fmt.Sprintf("%d", profile.Id)})
	ok, _, data := decodeAIGovernanceResponse(t, rec)
	require.True(t, ok)
	var detail struct {
		Risk struct {
			RiskLevel string `json:"risk_level"`
		} `json:"risk"`
	}
	require.NoError(t, common.Unmarshal(data, &detail))
	require.Equal(t, constant.RiskHigh, detail.Risk.RiskLevel, "固定底线必须 HIGH_RISK")
}

// fixture 辅助：创建主数据（domain/owner/usage/principal/purpose/app），返回各类 id。
type aiControllerFixture struct {
	DomainID    int
	OwnerID     int
	UsageID     int
	PrincipalID int
	PurposeID   int
	AppID       int
}

func setupAIGovernanceControllerFixture(t *testing.T) aiControllerFixture {
	t.Helper()
	f := aiControllerFixture{}
	_, _, dd := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateBusinessDomain, http.MethodPost, "/g", `{"domain_code":"finance","domain_name":"财务"}`, nil))
	var domain model.AIBusinessDomain
	require.NoError(t, common.Unmarshal(dd, &domain))
	f.DomainID = domain.Id

	_, _, dot := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateOwnerTeam, http.MethodPost, "/g", `{"team_code":"ai_application","team_name":"AI应用组"}`, nil))
	var owner model.AIOwnerTeam
	require.NoError(t, common.Unmarshal(dot, &owner))
	f.OwnerID = owner.Id

	_, _, dut := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateUsageTeam, http.MethodPost, "/g", `{"team_code":"finance_digital","team_name":"财务数字化组"}`, nil))
	var usage model.AIUsageTeam
	require.NoError(t, common.Unmarshal(dut, &usage))
	f.UsageID = usage.Id

	_, _, dp := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreatePrincipal, http.MethodPost, "/g", fmt.Sprintf(`{"principal_code":"zhangsan","principal_name":"张三","business_domain_id":%d,"usage_team_id":%d}`, domain.Id, usage.Id), nil))
	var principal model.AIPrincipal
	require.NoError(t, common.Unmarshal(dp, &principal))
	f.PrincipalID = principal.Id

	_, _, dpu := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateCredentialPurpose, http.MethodPost, "/g", `{"purpose_code":"workbuddy","purpose_name":"WorkBuddy","purpose_type":"DESKTOP_CLIENT"}`, nil))
	var purpose model.AICredentialPurpose
	require.NoError(t, common.Unmarshal(dpu, &purpose))
	f.PurposeID = purpose.Id

	_, _, da := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateApplication, http.MethodPost, "/g", fmt.Sprintf(`{"app_code":"hr_assistant","app_name":"人力助手","business_domain_id":%d,"owner_team_id":%d}`, domain.Id, owner.Id), nil))
	var app model.AIApplication
	require.NoError(t, common.Unmarshal(da, &app))
	f.AppID = app.Id
	return f
}

// 问题 10 + 9.11：Signing Keys GET 直接调用 handler，断言元数据字段且绝不含明文/密文。
func TestAIGovernanceSigningKeysGetHandlerMetaOnly(t *testing.T) {
	setupAIGovernanceControllerTestDB(t)
	t.Setenv(constant.AIAttributionMasterKeyEnv, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	f := setupAIGovernanceControllerFixture(t)

	tokenKey := "sk-signing-" + common.GetRandomString(20)
	tokenID := createAIGovernanceControllerToken(t, tokenKey)

	// DYNAMIC/PLATFORM disabled + 1 绑定。
	_, _, dpf := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateIdentityProfile, http.MethodPost, "/g", fmt.Sprintf(`{"token_id":%d,"identity_mode":"DYNAMIC","attribution_target_type":"PLATFORM","identity_assurance":"SIGNED_CONTEXT","caller_id":"workflow-prod","app_ids":[%d]}`, tokenID, f.AppID), nil))
	var profile model.AIIdentityProfile
	require.NoError(t, common.Unmarshal(dpf, &profile))

	// generate 只本次返回明文。
	_, _, dgen := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, GenerateSigningKey, http.MethodPost, "/g", "", map[string]string{"id": fmt.Sprintf("%d", profile.Id)}))
	var gen struct {
		Secret string `json:"secret"`
	}
	require.NoError(t, common.Unmarshal(dgen, &gen))
	require.NotEmpty(t, gen.Secret)

	// 直接调用 GetSigningKeys handler 并断言响应。
	rec := doAIGovernanceRequest(t, GetSigningKeys, http.MethodGet, "/g", "", map[string]string{"id": fmt.Sprintf("%d", profile.Id)})
	ok, msg, data := decodeAIGovernanceResponse(t, rec)
	require.True(t, ok, msg)
	body := string(data)
	require.Contains(t, body, "key_id", "GET 必须返回 key_id 元数据")
	require.Contains(t, body, "status", "GET 必须返回 status 元数据")
	require.Contains(t, body, "not_before")
	require.Contains(t, body, "expires_at")
	require.NotContains(t, body, gen.Secret, "GET 不得返回明文 secret")
	require.NotContains(t, body, "secret_ciphertext", "GET 不得暴露密文字段")
	require.NotContains(t, body, "ciphertext", "GET 不得泄露密文")
}

// 问题 9.11：列表返回详情（Principal/Purpose code/name、bindings、risk），且不含 token key/密钥密文。
func TestAIGovernanceListProfilesReturnsDetailNoSecret(t *testing.T) {
	setupAIGovernanceControllerTestDB(t)
	f := setupAIGovernanceControllerFixture(t)

	tokenKey := "sk-list-" + common.GetRandomString(20)
	tokenID := createAIGovernanceControllerToken(t, tokenKey)

	// 创建 disabled STATIC/PRINCIPAL Profile，再启用。
	_, _, dpf := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateIdentityProfile, http.MethodPost, "/g", fmt.Sprintf(`{"token_id":%d,"identity_mode":"STATIC","attribution_target_type":"PRINCIPAL","identity_assurance":"CREDENTIAL_ONLY","principal_id":%d,"credential_purpose_id":%d,"environment":"prod"}`, tokenID, f.PrincipalID, f.PurposeID), nil))
	var profile model.AIIdentityProfile
	require.NoError(t, common.Unmarshal(dpf, &profile))
	require.False(t, profile.Enabled)

	_, _, den := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, UpdateIdentityProfile, http.MethodPut, "/g", `{"enabled":true}`, map[string]string{"id": fmt.Sprintf("%d", profile.Id)}))
	var enabledProfile model.AIIdentityProfile
	require.NoError(t, common.Unmarshal(den, &enabledProfile))
	require.True(t, enabledProfile.Enabled)

	rec := doAIGovernanceRequest(t, GetIdentityProfiles, http.MethodGet, "/g", "", nil)
	ok, msg, data := decodeAIGovernanceResponse(t, rec)
	require.True(t, ok, msg)
	var page struct {
		Items []json.RawMessage `json:"items"`
		Total int64             `json:"total"`
	}
	require.NoError(t, common.Unmarshal(data, &page))
	require.Equal(t, int64(1), page.Total)
	require.Len(t, page.Items, 1)

	body := string(page.Items[0])
	require.Contains(t, body, "principal_code")
	require.Contains(t, body, "principal_name")
	require.Contains(t, body, "credential_purpose_code")
	require.Contains(t, body, "credential_purpose_name")
	require.Contains(t, body, "token_id")
	require.Contains(t, body, "risk")
	require.Contains(t, body, "rate_limits")
	require.Contains(t, body, "identity_mode")
	require.Contains(t, body, "attribution_target_type")
	require.Contains(t, body, "identity_assurance")
	require.NotContains(t, body, tokenKey, "列表详情不得返回 Token Key")
	require.NotContains(t, body, "secret_ciphertext", "列表详情不得暴露密钥密文")
}

// 审查问题 5：详情引用缺失必须返回错误，不得静默返回半详情。
func TestAIGovernanceProfileDetailMissingRefError(t *testing.T) {
	setupAIGovernanceControllerTestDB(t)
	f := setupAIGovernanceControllerFixture(t)
	tokenID := createAIGovernanceControllerToken(t, "sk-miss-"+common.GetRandomString(20))

	_, _, dpf := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateIdentityProfile, http.MethodPost, "/g", fmt.Sprintf(`{"token_id":%d,"identity_mode":"STATIC","attribution_target_type":"PRINCIPAL","identity_assurance":"CREDENTIAL_ONLY","principal_id":%d,"credential_purpose_id":%d}`, tokenID, f.PrincipalID, f.PurposeID), nil))
	var profile model.AIIdentityProfile
	require.NoError(t, common.Unmarshal(dpf, &profile))

	// 删除被引用的 principal 行。
	require.NoError(t, model.DB.Delete(&model.AIPrincipal{}, f.PrincipalID).Error)

	rec := doAIGovernanceRequest(t, GetIdentityProfile, http.MethodGet, "/g", "", map[string]string{"id": fmt.Sprintf("%d", profile.Id)})
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	require.False(t, payload.Success, "引用缺失必须返回错误而非半详情")
	require.Contains(t, payload.Message, "使用主体", "错误信息必须指向缺失的引用")
}

// 门禁：GetIdentityAuditEvents 对非法 token_id / profile_id 必须返回 success=false，
// 不得静默吞掉查询参数解析错误（否则查询过滤被悄悄忽略）。
func TestAIGovernanceAuditEventsInvalidQueryRejected(t *testing.T) {
	setupAIGovernanceControllerTestDB(t)
	createAIGovernanceControllerToken(t, "sk-audit-"+common.GetRandomString(20))

	for _, tc := range []struct {
		name  string
		query string
	}{
		{"token_id_bad", "token_id=bad"},
		{"profile_id_bad", "profile_id=bad"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := doAIGovernanceRequest(t, GetIdentityAuditEvents, http.MethodGet, "/g?"+tc.query, "", nil)
			require.Equal(t, http.StatusOK, rec.Code)
			var payload struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
			require.False(t, payload.Success, "%s 必须被拒绝而非静默吞掉", tc.query)
			require.Contains(t, payload.Message, "非法", "%s 错误信息须含『非法』: %s", tc.query, payload.Message)
		})
	}
}

// D.2：GetIdentityAuditEvents 稳定排序为 created_at DESC, id DESC，保证同一时间戳的
// 多条事件顺序确定（分页不因时间戳相同而在翻页间抖动）。
func TestAIGovernanceAuditEventsDeterministicOrdering(t *testing.T) {
	setupAIGovernanceControllerTestDB(t)
	const ts = int64(1700000000)
	for i := 0; i < 3; i++ {
		require.NoError(t, model.DB.Create(&model.AIIdentityAuditEvent{
			CreatedAt:   ts,
			RequestId:   fmt.Sprintf("req-order-%d", i),
			Result:      constant.IdentityAuditResultUnverified,
			ReasonCode:  constant.ReasonCodeProfileRequired,
			HttpMethod:  "POST",
			RequestPath: "/v1/chat/completions",
			ClientIp:    "203.0.113.1",
		}).Error)
	}
	rec := doAIGovernanceRequest(t, GetIdentityAuditEvents, http.MethodGet, "/g", "", nil)
	ok, msg, data := decodeAIGovernanceResponse(t, rec)
	require.True(t, ok, msg)
	var page struct {
		Items []model.AIIdentityAuditEvent `json:"items"`
		Total int64                        `json:"total"`
	}
	require.NoError(t, common.Unmarshal(data, &page))
	require.Equal(t, int64(3), page.Total)
	require.Len(t, page.Items, 3)
	// 相同 created_at → 必须按 id DESC 返回，保证确定性顺序。
	require.Greater(t, page.Items[0].Id, page.Items[1].Id, "同时间戳首条应为最大 id")
	require.Greater(t, page.Items[1].Id, page.Items[2].Id, "同时间戳按 id DESC")
}

// 门禁：分页 page_size 有 200 上限，超大值必须在 handler 实际返回中被限制；
// 默认 20、合法小值不被改写。通过 handler 返回的 page_size 保护 API 合约。
func TestAIGovernancePaginationPageSizeCapped(t *testing.T) {
	setupAIGovernanceControllerTestDB(t)
	// 造一条数据，确保列表返回结构可解析。
	_, _, dd := decodeAIGovernanceResponse(t, doAIGovernanceRequest(t, CreateBusinessDomain, http.MethodPost, "/g", `{"domain_code":"finance","domain_name":"财务"}`, nil))
	var domain model.AIBusinessDomain
	require.NoError(t, common.Unmarshal(dd, &domain))
	_ = domain

	// 超大 page_size 必须被限制到 200。
	rec := doAIGovernanceRequest(t, GetBusinessDomains, http.MethodGet, "/g?page_size=5000", "", nil)
	ok, msg, data := decodeAIGovernanceResponse(t, rec)
	require.True(t, ok, msg)
	var page struct {
		PageSize int `json:"page_size"`
	}
	require.NoError(t, common.Unmarshal(data, &page))
	require.Equal(t, 200, page.PageSize, "page_size 必须被限制到 200 上限")

	// 合法小值不被改写。
	rec = doAIGovernanceRequest(t, GetBusinessDomains, http.MethodGet, "/g?page_size=5", "", nil)
	ok, msg, data = decodeAIGovernanceResponse(t, rec)
	require.True(t, ok, msg)
	require.NoError(t, common.Unmarshal(data, &page))
	require.Equal(t, 5, page.PageSize)

	// 默认 page_size=20。
	rec = doAIGovernanceRequest(t, GetBusinessDomains, http.MethodGet, "/g", "", nil)
	ok, msg, data = decodeAIGovernanceResponse(t, rec)
	require.True(t, ok, msg)
	require.NoError(t, common.Unmarshal(data, &page))
	require.Equal(t, 20, page.PageSize)
}
