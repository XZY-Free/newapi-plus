package model

// 第三批“消费事实归因与可查询日志”回归测试（冻结方案 V1.1 第 8 章）。
// 只验证可观察契约。

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 基础设施：构造完整 Gin Context + 清理 logs / 恢复全局开关
// ---------------------------------------------------------------------------

func newAttrGinContext(t *testing.T, trusted *constant.TrustedAttributionContext, username, requestID string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("username", username)
	ctx.Set(common.RequestIdKey, requestID)
	if trusted != nil {
		common.SetTrustedAttribution(ctx, trusted)
	}
	return ctx
}

// withLogCleanup 清理 logs 表并恢复全局 LogConsumeEnabled。
func withLogCleanup(t *testing.T) {
	t.Helper()
	prev := common.LogConsumeEnabled
	t.Cleanup(func() {
		DB.Exec("DELETE FROM logs")
		common.LogConsumeEnabled = prev
	})
}

func parseOther(t *testing.T, other string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(other, &m), "Other must be valid JSON")
	return m
}

func aiAttribution(t *testing.T, other string) map[string]interface{} {
	t.Helper()
	m := parseOther(t, other)
	a, ok := m["ai_attribution"].(map[string]interface{})
	require.True(t, ok, "Other must contain ai_attribution object")
	return a
}

func recordConsume(t *testing.T, ctx *gin.Context, other map[string]interface{}) Log {
	t.Helper()
	RecordConsumeLog(ctx, 4242, RecordConsumeLogParams{
		ChannelId:        7,
		PromptTokens:     100,
		CompletionTokens: 50,
		ModelName:        "test-model",
		TokenName:        "test-token",
		Quota:            123456,
		Content:          "hello",
		TokenId:          999,
		UseTimeSeconds:   3,
		IsStream:         false,
		Group:            "default-group",
		Other:            other,
	})
	reqID := ctx.GetString(common.RequestIdKey)
	var log Log
	require.NoError(t, DB.Where("request_id = ?", reqID).Order("id desc").First(&log).Error)
	return log
}

func recordError(t *testing.T, ctx *gin.Context, other map[string]interface{}) Log {
	t.Helper()
	RecordErrorLog(ctx, 4242, 7, "test-model", "test-token", "boom", 999, 2, false, "default-group", other)
	reqID := ctx.GetString(common.RequestIdKey)
	var log Log
	require.NoError(t, DB.Where("request_id = ?", reqID).Order("id desc").First(&log).Error)
	return log
}

// commonSnapshotFields 是 8.3 通用字段，必须稳定出现（含 false 布尔与空 failure_reason）。
var commonSnapshotFields = []string{
	"profile_id",
	"token_id",
	"environment",
	"identity_mode",
	"attribution_target_type",
	"identity_assurance",
	"identity_source",
	"identity_verified",
	"credential_verified",
	"client_verified",
	"failure_reason",
}

func assertCommonFields(t *testing.T, a map[string]interface{}) {
	t.Helper()
	for _, k := range commonSnapshotFields {
		_, ok := a[k]
		assert.Truef(t, ok, "common field %q must be present", k)
	}
}

// assertSecretsAbsent 校验快照不出现秘密字段或秘密值。
func assertSecretsAbsent(t *testing.T, a map[string]interface{}) {
	t.Helper()
	secrets := []string{"signing_secret", "signature", "nonce", "api_key", "signing_secret_plaintext"}
	for _, k := range secrets {
		_, ok := a[k]
		assert.Falsef(t, ok, "forbidden secret field %q must NOT appear", k)
	}
	// 序列化后的快照不得包含任何测试秘密标记值。
	raw := common.MapToJsonStr(a)
	for _, marker := range []string{"SUPER-SECRET", "sk-forged-secret"} {
		assert.Falsef(t, strings.Contains(raw, marker), "secret marker %q must not leak", marker)
	}
}

// ---------------------------------------------------------------------------
// 1. STATIC + PRINCIPAL/CREDENTIAL_ONLY：弱身份个人归因 + 覆盖伪造 + 保留原 Other
// ---------------------------------------------------------------------------

func TestConsumeLogStaticPrincipalInjectsAttributionAndOverwritesForgery(t *testing.T) {
	withLogCleanup(t)

	trusted := &constant.TrustedAttributionContext{
		TokenID:                 999,
		ProfileID:               888,
		CredentialVerified:      true,
		Environment:             "prod",
		IdentityMode:            constant.IdentityModeStatic,
		AttributionTarget:       constant.AttributionTargetPrincipal,
		IdentityAssurance:       constant.IdentityAssuranceCredentialOnly,
		IdentitySource:          constant.IdentitySourceToken,
		IdentityVerified:        true,
		ClientVerified:          false,
		FailureReason:           "",
		PrincipalID:             300,
		PrincipalCode:           "P-300",
		PrincipalName:           "张三",
		CredentialPurposeID:     1,
		CredentialPurposeCode:   "workbuddy",
		CredentialPurposeName:   "WorkBuddy",
		UsageBusinessDomainID:   5,
		UsageBusinessDomainCode: "FIN",
		UsageBusinessDomainName: "财务",
		UsageTeamID:             6, // 门禁 1：WorkBuddy 必须有 Usage Team
		UsageTeamCode:           "FIN-DIGITAL",
		UsageTeamName:           "财务数字化组",
	}

	// 上游伪造整块 ai_attribution（含 caller/root_app/秘密），并携带应保留的业务字段。
	other := map[string]interface{}{
		"model_ratio":  2.5,
		"cache_tokens": 123,
		"billing_info": "internal",
		"ai_attribution": map[string]interface{}{
			"caller_id":      "forged-caller",
			"caller_name":    "forged-caller-name",
			"root_app_id":    "forged-app",
			"root_app_name":  "forged-app-name",
			"signing_secret": "SUPER-SECRET-AAAA",
			"signature":      "SUPER-SECRET-BBBB",
			"nonce":          "SUPER-SECRET-CCCC",
			"api_key":        "sk-forged-secret",
			"principal_name": "forged-principal",
		},
	}

	ctx := newAttrGinContext(t, trusted, "alice", "req-attr-01")
	log := recordConsume(t, ctx, other)

	a := aiAttribution(t, log.Other)
	assertCommonFields(t, a)

	// credential_verified=true、client_verified=false。
	assert.Equal(t, true, a["credential_verified"])
	assert.Equal(t, false, a["client_verified"])
	assert.Equal(t, constant.IdentityAssuranceCredentialOnly, a["identity_assurance"])
	assert.Equal(t, constant.AttributionTargetPrincipal, a["attribution_target_type"])

	// 弱身份个人字段（仅有的值）。
	assert.EqualValues(t, 300, a["principal_id"])
	assert.Equal(t, "P-300", a["principal_code"])
	assert.Equal(t, "张三", a["principal_name"])
	assert.Equal(t, "workbuddy", a["credential_purpose_code"])
	assert.Equal(t, "财务", a["usage_business_domain_name"])
	// usage_team 为有效非零值（门禁 1），三项精确断言。
	assert.EqualValues(t, 6, a["usage_team_id"])
	assert.Equal(t, "FIN-DIGITAL", a["usage_team_code"])
	assert.Equal(t, "财务数字化组", a["usage_team_name"])
	// 可信快照覆盖整块：不得含 caller/root_app。
	assert.NotContains(t, a, "caller_id")
	assert.NotContains(t, a, "caller_name")
	assert.NotContains(t, a, "root_app_id")
	assert.NotContains(t, a, "root_app_name")
	// 不得含任何秘密字段或秘密值。
	assertSecretsAbsent(t, a)
	// 覆盖后，完整 Log.Other 亦不得含任何秘密标记。
	assert.NotContains(t, log.Other, "SUPER-SECRET", "full Other must not leak SUPER-SECRET")
	assert.NotContains(t, log.Other, "sk-forged-secret", "full Other must not leak sk-forged-secret")

	// 原 Other 的业务字段保持。
	m := parseOther(t, log.Other)
	assert.Equal(t, 2.5, m["model_ratio"])
	assert.EqualValues(t, 123, m["cache_tokens"])
	assert.Equal(t, "internal", m["billing_info"])
}

// ---------------------------------------------------------------------------
// 2. STATIC + APPLICATION：App Domain/Owner Team，client_verified=false，无 caller/执行/signing key
// ---------------------------------------------------------------------------

func TestConsumeLogStaticApplicationSavesAppDomainAndOwnerTeam(t *testing.T) {
	withLogCleanup(t)

	trusted := &constant.TrustedAttributionContext{
		TokenID:                       999,
		ProfileID:                     888,
		CredentialVerified:            true,
		Environment:                   "prod",
		IdentityMode:                  constant.IdentityModeStatic,
		AttributionTarget:             constant.AttributionTargetApplication,
		IdentityAssurance:             constant.IdentityAssuranceCredentialOnly,
		IdentitySource:                constant.IdentitySourceToken,
		IdentityVerified:              true,
		ClientVerified:                false,
		RootAppID:                     "hr_assistant",
		RootAppName:                   "HR助手",
		ApplicationBusinessDomainID:   10,
		ApplicationBusinessDomainCode: "HR",
		ApplicationBusinessDomainName: "人力",
		OwnerTeamID:                   20,
		OwnerTeamCode:                 "AI-APP",
		OwnerTeamName:                 "AI应用组",
		// caller / 执行 / signing_key 全空。
	}

	ctx := newAttrGinContext(t, trusted, "bob", "req-attr-02")
	log := recordConsume(t, ctx, map[string]interface{}{"model_ratio": 1.0})

	a := aiAttribution(t, log.Other)
	assertCommonFields(t, a)
	assert.Equal(t, false, a["client_verified"])
	assert.Equal(t, constant.AttributionTargetApplication, a["attribution_target_type"])

	assert.Equal(t, "hr_assistant", a["root_app_id"])
	assert.Equal(t, "HR助手", a["root_app_name"])
	assert.EqualValues(t, 10, a["application_business_domain_id"])
	assert.Equal(t, "HR", a["application_business_domain_code"])
	assert.Equal(t, "人力", a["application_business_domain_name"])
	assert.EqualValues(t, 20, a["owner_team_id"])
	assert.Equal(t, "AI-APP", a["owner_team_code"])
	assert.Equal(t, "AI应用组", a["owner_team_name"])

	// 不得出现 caller / 执行 / signing key。
	assert.NotContains(t, a, "caller_id")
	assert.NotContains(t, a, "root_run_id")
	assert.NotContains(t, a, "current_execution_id")
	assert.NotContains(t, a, "signing_key_id")
	assertSecretsAbsent(t, a)
}

// ---------------------------------------------------------------------------
// 3. DYNAMIC/HYBRID 强身份：caller/app/run/current execution/signing_key_id，client_verified=true
// ---------------------------------------------------------------------------

func TestConsumeLogStrongIdentitySavesCallerAppRunAndKeyID(t *testing.T) {
	withLogCleanup(t)

	cases := []struct {
		name           string
		mode           string
		assurance      string
		source         string
		attributionTgt string
	}{
		{
			name:           "DYNAMIC + SIGNED_CONTEXT",
			mode:           constant.IdentityModeDynamic,
			assurance:      constant.IdentityAssuranceSignedContext,
			source:         constant.IdentitySourceSignedContext,
			attributionTgt: constant.AttributionTargetPlatform,
		},
		{
			name:           "HYBRID + HYBRID_VERIFIED_CONTEXT",
			mode:           constant.IdentityModeHybrid,
			assurance:      constant.IdentityAssuranceHybridVerified,
			source:         constant.IdentitySourceHybrid,
			attributionTgt: constant.AttributionTargetPlatform,
		},
	}

	for i, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			trusted := &constant.TrustedAttributionContext{
				TokenID:                       999,
				ProfileID:                     888,
				CredentialVerified:            true,
				Environment:                   "prod",
				IdentityMode:                  tt.mode,
				AttributionTarget:             tt.attributionTgt,
				IdentityAssurance:             tt.assurance,
				IdentitySource:                tt.source,
				IdentityVerified:              true,
				ClientVerified:                true,
				CallerID:                      "workflow-platform-prod",
				CallerName:                    "工作流平台",
				RootAppID:                     "hr_assistant",
				RootAppName:                   "HR助手",
				ApplicationBusinessDomainID:   10,
				ApplicationBusinessDomainCode: "HR",
				ApplicationBusinessDomainName: "人力",
				OwnerTeamID:                   20,
				OwnerTeamCode:                 "AI-APP",
				OwnerTeamName:                 "AI应用组",
				RootRunID:                     "run_xxx",
				CurrentExecutionID:            "exec_1",
				ParentExecutionID:             "exec_0",
				ExecutionType:                 "workflow",
				ExecutionDepth:                2,
				WorkflowID:                    "wf_7",
				AgentID:                       "agent_9",
				TaskID:                        "task_1",
				NodeID:                        "node_2",
				SigningKeyID:                  "sk-verified-1",
				// signing_secret/signature/nonce/context 不承载于 Trusted Context。
			}

			reqID := fmt.Sprintf("req-attr-03-%d", i)
			ctx := newAttrGinContext(t, trusted, "carol", reqID)
			log := recordConsume(t, ctx, map[string]interface{}{"cache_tokens": 9})

			a := aiAttribution(t, log.Other)
			assertCommonFields(t, a)
			assert.Equal(t, true, a["client_verified"], "client_verified=true for strong identity")
			assert.Equal(t, tt.assurance, a["identity_assurance"])
			assert.Equal(t, tt.mode, a["identity_mode"])

			// caller / app / run / current execution / signing_key_id。
			assert.Equal(t, "workflow-platform-prod", a["caller_id"])
			assert.Equal(t, "工作流平台", a["caller_name"])
			assert.Equal(t, "hr_assistant", a["root_app_id"])
			assert.Equal(t, "HR助手", a["root_app_name"])
			assert.Equal(t, "run_xxx", a["root_run_id"])
			assert.Equal(t, "exec_1", a["current_execution_id"])
			assert.Equal(t, "exec_0", a["parent_execution_id"])
			assert.EqualValues(t, 2, a["execution_depth"])
			assert.Equal(t, "wf_7", a["workflow_id"])
			assert.Equal(t, "agent_9", a["agent_id"])
			assert.Equal(t, "task_1", a["task_id"])
			assert.Equal(t, "node_2", a["node_id"])
			assert.Equal(t, "sk-verified-1", a["signing_key_id"])

			// application_business_domain 与 owner_team 的 id/code/name。
			assert.EqualValues(t, 10, a["application_business_domain_id"])
			assert.Equal(t, "HR", a["application_business_domain_code"])
			assert.Equal(t, "人力", a["application_business_domain_name"])
			assert.EqualValues(t, 20, a["owner_team_id"])
			assert.Equal(t, "AI-APP", a["owner_team_code"])
			assert.Equal(t, "AI应用组", a["owner_team_name"])

			assertSecretsAbsent(t, a)
			assert.NotContains(t, log.Other, "SUPER-SECRET")
			assert.NotContains(t, log.Other, "sk-forged-secret")
		})
	}
}

// ---------------------------------------------------------------------------
// 4. RecordErrorLog 写相同 ai_attribution 并覆盖伪造值
// ---------------------------------------------------------------------------

func TestErrorLogInjectsAttributionAndOverwritesForgery(t *testing.T) {
	withLogCleanup(t)

	trusted := &constant.TrustedAttributionContext{
		TokenID:               999,
		ProfileID:             888,
		CredentialVerified:    true,
		Environment:           "prod",
		IdentityMode:          constant.IdentityModeStatic,
		AttributionTarget:     constant.AttributionTargetPrincipal,
		IdentityAssurance:     constant.IdentityAssuranceCredentialOnly,
		IdentitySource:        constant.IdentitySourceToken,
		IdentityVerified:      true,
		ClientVerified:        false,
		PrincipalID:           300,
		PrincipalCode:         "P-300",
		PrincipalName:         "张三",
		CredentialPurposeCode: "workbuddy",
	}

	other := map[string]interface{}{
		"ai_attribution": map[string]interface{}{
			"caller_id":      "forged-caller",
			"root_app_id":    "forged-app",
			"signing_secret": "SUPER-SECRET-DDDD",
		},
		"upstream": "keep-me",
	}

	ctx := newAttrGinContext(t, trusted, "dave", "req-attr-04")
	log := recordError(t, ctx, other)

	a := aiAttribution(t, log.Other)
	assertCommonFields(t, a)
	assert.Equal(t, "张三", a["principal_name"])
	assert.Equal(t, true, a["credential_verified"])
	assert.NotContains(t, a, "caller_id")
	assert.NotContains(t, a, "root_app_id")
	assertSecretsAbsent(t, a)
	// 覆盖后，完整 Log.Other 亦不得含任何秘密标记。
	assert.NotContains(t, log.Other, "SUPER-SECRET", "full Other must not leak SUPER-SECRET")
	assert.NotContains(t, log.Other, "sk-forged-secret", "full Other must not leak sk-forged-secret")

	// 无关字段保留。
	m := parseOther(t, log.Other)
	assert.Equal(t, "keep-me", m["upstream"])
}

// ---------------------------------------------------------------------------
// 5. 无 Trusted Context 的 legacy 日志：不得凭空生成 ai_attribution
// ---------------------------------------------------------------------------

func TestConsumeLogWithoutTrustedContextIsLegacyUnattributed(t *testing.T) {
	withLogCleanup(t)

	other := map[string]interface{}{"model_ratio": 0.5, "custom": "x"}
	ctx := newAttrGinContext(t, nil, "eve", "req-attr-05")
	log := recordConsume(t, ctx, other)

	m := parseOther(t, log.Other)
	assert.NotContains(t, m, "ai_attribution", "legacy 日志不得被解释为验证失败或凭空生成快照")
	assert.Equal(t, 0.5, m["model_ratio"])
	assert.Equal(t, "x", m["custom"])
}

// ---------------------------------------------------------------------------
// 6. 历史快照：调用时快照，主数据迁移只影响未来
// ---------------------------------------------------------------------------

func TestHistorySnapshotKeepsOldValueAfterTrustedContextMutation(t *testing.T) {
	withLogCleanup(t)

	// 旧主数据：先用旧 Principal/App 名称记录。
	oldCtx := &constant.TrustedAttributionContext{
		TokenID:            999,
		ProfileID:          888,
		CredentialVerified: true,
		Environment:        "prod",
		IdentityMode:       constant.IdentityModeStatic,
		AttributionTarget:  constant.AttributionTargetApplication,
		IdentityAssurance:  constant.IdentityAssuranceCredentialOnly,
		IdentitySource:     constant.IdentitySourceToken,
		IdentityVerified:   true,
		RootAppID:          "hr_assistant",
		RootAppName:        "旧应用名",
		OwnerTeamName:      "旧团队名",
	}
	ctxOld := newAttrGinContext(t, oldCtx, "frank", "req-attr-06a")
	oldLog := recordConsume(t, ctxOld, map[string]interface{}{"model_ratio": 1.0})

	// 修改内存中的 Trusted Context 后记录第二条。
	oldCtx.RootAppName = "新应用名"
	oldCtx.OwnerTeamName = "新团队名"
	ctxNew := newAttrGinContext(t, oldCtx, "frank", "req-attr-06b")
	newLog := recordConsume(t, ctxNew, map[string]interface{}{"model_ratio": 2.0})

	oldAttr := aiAttribution(t, oldLog.Other)
	newAttr := aiAttribution(t, newLog.Other)

	// 旧日志仍为旧值。
	assert.Equal(t, "旧应用名", oldAttr["root_app_name"])
	assert.Equal(t, "旧团队名", oldAttr["owner_team_name"])
	// 新日志为新值。
	assert.Equal(t, "新应用名", newAttr["root_app_name"])
	assert.Equal(t, "新团队名", newAttr["owner_team_name"])
}

// ---------------------------------------------------------------------------
// 7. 不修改 QuotaData/统计：Consume Log 原有 Log 字段保持准确
// ---------------------------------------------------------------------------

func TestConsumeLogKeepsQuotaTokenChannelGroupAccurate(t *testing.T) {
	withLogCleanup(t)

	trusted := &constant.TrustedAttributionContext{
		TokenID:            999,
		ProfileID:          888,
		CredentialVerified: true,
		Environment:        "prod",
		IdentityMode:       constant.IdentityModeStatic,
		AttributionTarget:  constant.AttributionTargetPrincipal,
		IdentityAssurance:  constant.IdentityAssuranceCredentialOnly,
		IdentitySource:     constant.IdentitySourceToken,
		IdentityVerified:   true,
	}
	ctx := newAttrGinContext(t, trusted, "grace", "req-attr-07")
	RecordConsumeLog(ctx, 4242, RecordConsumeLogParams{
		ChannelId:        42,
		PromptTokens:     111,
		CompletionTokens: 222,
		ModelName:        "m",
		TokenName:        "tn",
		Quota:            999999,
		Content:          "c",
		TokenId:          777,
		UseTimeSeconds:   5,
		IsStream:         true,
		Group:            "grp-7",
		Other:            map[string]interface{}{"model_ratio": 0.5},
	})

	reqID := ctx.GetString(common.RequestIdKey)
	var log Log
	require.NoError(t, DB.Where("request_id = ?", reqID).Order("id desc").First(&log).Error)

	assert.Equal(t, LogTypeConsume, log.Type)
	assert.EqualValues(t, 42, log.ChannelId)
	assert.EqualValues(t, 111, log.PromptTokens)
	assert.EqualValues(t, 222, log.CompletionTokens)
	assert.EqualValues(t, 999999, log.Quota)
	assert.EqualValues(t, 777, log.TokenId)
	assert.Equal(t, "grp-7", log.Group)
	assert.Equal(t, "grace", log.Username)
	assert.Equal(t, "tn", log.TokenName)
	assert.Equal(t, "m", log.ModelName)
	assert.Equal(t, true, log.IsStream)

	// ai_attribution 仅作归因补充，不影响上述 Log 字段。
	_ = aiAttribution(t, log.Other)
}
