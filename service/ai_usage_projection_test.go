package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

// other 由 ai_attribution 快照 + 可选 task_id 构成。
func withAttribution(attr map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"ai_attribution": attr}
}

// §12.6 / E.2 P1-C：模型请求事实（无 task_id）正常计数，时长由秒换算为毫秒。
func TestBuildUsageProjectionRowSuccess(t *testing.T) {
	attribution := map[string]interface{}{
		"profile_id": 9, "principal_id": 1, "credential_purpose_id": 10,
		"usage_business_domain_id": 3, "usage_team_id": 55,
		"identity_assurance": "CREDENTIAL_ONLY", "client_verified": false,
	}
	log := &model.Log{
		CreatedAt: 1750000000, ModelName: "gpt-4o", Type: model.LogTypeConsume,
		PromptTokens: 100, CompletionTokens: 50, Quota: 3000, UseTime: 1200,
	}
	row, ok := BuildUsageProjectionRow(log, withAttribution(attribution), bucketHour(log.CreatedAt))
	if !ok {
		t.Fatal("应生成投影行")
	}
	if row.BucketTime != bucketHour(1750000000) {
		t.Fatalf("bucket_time 应归一为整点: %d", row.BucketTime)
	}
	if row.RequestCount != 1 || row.SuccessCount != 1 || row.ErrorCount != 0 {
		t.Fatalf("计数错误: req=%d ok=%d err=%d", row.RequestCount, row.SuccessCount, row.ErrorCount)
	}
	if row.InputTokens != 100 || row.OutputTokens != 50 || row.TotalTokens != 150 {
		t.Fatalf("token 错误: %+v", row)
	}
	// P1-F：UseTime 单位为秒，DurationMsTotal 为毫秒。
	if row.QuotaNet != 3000 || row.DurationMsTotal != 1200*1000 {
		t.Fatalf("quota/duration 错误: %+v", row)
	}
	if row.ProfileID != 9 || row.CredentialPurposeID != 10 || row.UsageTeamID != 55 {
		t.Fatalf("维度错误: %+v", row)
	}
}

// §12.6：错误日志归入 error 计数。
func TestBuildUsageProjectionRowError(t *testing.T) {
	attribution := map[string]interface{}{"profile_id": 9, "credential_purpose_id": 10}
	log := &model.Log{CreatedAt: 1750000000, Type: model.LogTypeError, PromptTokens: 10, Quota: 500}
	row, ok := BuildUsageProjectionRow(log, withAttribution(attribution), bucketHour(log.CreatedAt))
	if !ok {
		t.Fatal("应生成投影行")
	}
	if row.ErrorCount != 1 || row.SuccessCount != 0 {
		t.Fatalf("错误日志应计入 error: %+v", row)
	}
}

// §12.6：无归因快照的日志不产生投影行。
func TestBuildUsageProjectionRowNoAttribution(t *testing.T) {
	log := &model.Log{CreatedAt: 1750000000, Type: model.LogTypeConsume, PromptTokens: 10}
	if _, ok := BuildUsageProjectionRow(log, map[string]interface{}{}, bucketHour(log.CreatedAt)); ok {
		t.Fatal("无归因不应产生投影行")
	}
}

// E.2 P1-C：任务差额结算（正差额补扣）不制造模型请求，Quota 为净 +。
func TestBuildUsageProjectionRowTaskBillingRecalc(t *testing.T) {
	attribution := map[string]interface{}{"profile_id": 9, "credential_purpose_id": 10, "identity_assurance": "CREDENTIAL_ONLY"}
	other := withAttribution(attribution)
	other["task_id"] = "task-abc"
	log := &model.Log{CreatedAt: 1750000000, Type: model.LogTypeConsume, PromptTokens: 10, Quota: 500}
	row, ok := BuildUsageProjectionRow(log, other, bucketHour(log.CreatedAt))
	if !ok {
		t.Fatal("应生成投影行")
	}
	if row.RequestCount != 0 || row.SuccessCount != 0 || row.ErrorCount != 0 {
		t.Fatalf("Task Billing 调整不得累计请求: req=%d ok=%d err=%d", row.RequestCount, row.SuccessCount, row.ErrorCount)
	}
	if row.QuotaNet != 500 {
		t.Fatalf("补扣净额应为 +500, got %d", row.QuotaNet)
	}
}

// E.2 P1-C：任务退款不制造模型请求，Quota 为净 -（不得把退款正数再累计到 quota_net）。
func TestBuildUsageProjectionRowTaskBillingRefund(t *testing.T) {
	attribution := map[string]interface{}{"profile_id": 9, "credential_purpose_id": 10, "identity_assurance": "CREDENTIAL_ONLY"}
	other := withAttribution(attribution)
	other["task_id"] = "task-abc"
	log := &model.Log{CreatedAt: 1750000000, Type: model.LogTypeRefund, PromptTokens: 0, Quota: 300}
	row, ok := BuildUsageProjectionRow(log, other, bucketHour(log.CreatedAt))
	if !ok {
		t.Fatal("应生成投影行")
	}
	if row.RequestCount != 0 || row.SuccessCount != 0 || row.ErrorCount != 0 {
		t.Fatalf("Task Billing 退款不得累计请求: req=%d ok=%d err=%d", row.RequestCount, row.SuccessCount, row.ErrorCount)
	}
	if row.QuotaNet != -300 {
		t.Fatalf("退款净额应为 -300, got %d", row.QuotaNet)
	}
}

// E.2 P1-B：强身份（SIGNED_CONTEXT）但未验证的流量清空 Caller/App 维度并降级为 UNVERIFIED。
func TestBuildUsageProjectionRowStrongUnverifiedNormalized(t *testing.T) {
	attribution := map[string]interface{}{
		"profile_id":                9,
		"caller_id":                 "caller-9",
		"root_app_id":               "app-wb",
		"application_business_domain_id": 2,
		"owner_team_id":                   4,
		"identity_assurance":              "SIGNED_CONTEXT",
		"client_verified":                 false,
	}
	log := &model.Log{CreatedAt: 1750000000, ModelName: "gpt-4o", Type: model.LogTypeConsume, Quota: 100}
	row, ok := BuildUsageProjectionRow(log, withAttribution(attribution), bucketHour(log.CreatedAt))
	if !ok {
		t.Fatal("应生成投影行")
	}
	if row.ProfileID != 9 {
		t.Fatalf("保留 profile_id 供治理调查, got %d", row.ProfileID)
	}
	if row.CallerKey != "" || row.RootAppCode != "" || row.AppID != 0 ||
		row.AppBusinessDomainID != 0 || row.OwnerTeamID != 0 {
		t.Fatalf("未验证强身份应清空 Caller/App 维度: %+v", row)
	}
	if row.IdentityAssurance != "UNVERIFIED" {
		t.Fatalf("应降级为 UNVERIFIED, got %s", row.IdentityAssurance)
	}
}

// E.2 P1-B：CREDENTIAL_ONLY（固定 App 可信登记）不受未验证规范化影响。
func TestBuildUsageProjectionRowCredentialOnlyUnaffected(t *testing.T) {
	attribution := map[string]interface{}{
		"profile_id": 9, "credential_purpose_id": 10,
		"caller_id": "caller-9", "root_app_id": "app-wb",
		"application_business_domain_id": 2, "owner_team_id": 4,
		"identity_assurance": "CREDENTIAL_ONLY", "client_verified": false,
	}
	log := &model.Log{CreatedAt: 1750000000, ModelName: "gpt-4o", Type: model.LogTypeConsume, Quota: 100}
	row, ok := BuildUsageProjectionRow(log, withAttribution(attribution), bucketHour(log.CreatedAt))
	if !ok {
		t.Fatal("应生成投影行")
	}
	if row.CallerKey != "caller-9" || row.RootAppCode != "app-wb" {
		t.Fatalf("CREDENTIAL_ONLY 不应被清空 Caller/App: %+v", row)
	}
	if row.IdentityAssurance != "CREDENTIAL_ONLY" {
		t.Fatalf("CREDENTIAL_ONLY 不应被降级, got %s", row.IdentityAssurance)
	}
}

// §12.5：median 中位数正确（偶数取中间两数均值）。
func TestMedian(t *testing.T) {
	if got := median([]int64{3, 1, 2}); got != 2 {
		t.Fatalf("中位数应为 2, got %d", got)
	}
	if got := median([]int64{10, 20}); got != 15 {
		t.Fatalf("偶数中位数应为 15, got %d", got)
	}
	if got := median([]int64{}); got != 0 {
		t.Fatalf("空切片中位数应为 0, got %d", got)
	}
}

// §12.5：hourOfDay 按整点对 24 取模（用于同一小时基线对齐）。
func TestHourOfDay(t *testing.T) {
	if got := hourOfDay(1750000000); got != (1750000000/3600)%24 {
		t.Fatalf("hourOfDay 计算错误: %d", got)
	}
}

// §12.5：collectCandidateRows 将同一 profile+purpose+model+hour 的多桶归并。
func TestCollectCandidateRows(t *testing.T) {
	rows := []*model.AIUsageHourly{
		{ProfileID: 9, PrincipalID: 1, CredentialPurposeID: 10, ModelName: "gpt", BucketTime: 100, RequestCount: 2, TotalTokens: 20, QuotaNet: 10},
		{ProfileID: 9, PrincipalID: 1, CredentialPurposeID: 10, ModelName: "gpt", BucketTime: 100, RequestCount: 3, TotalTokens: 30, QuotaNet: 5},
		{ProfileID: 9, PrincipalID: 2, CredentialPurposeID: 10, ModelName: "gpt", BucketTime: 100, RequestCount: 1, TotalTokens: 1, QuotaNet: 1},
	}
	cands := collectCandidateRows(rows)
	if len(cands) != 2 {
		t.Fatalf("应归并为 2 个候选, got %d", len(cands))
	}
	for _, c := range cands {
		if c.PrincipalID == 1 && (c.RequestCount != 5 || c.TotalTokens != 50) {
			t.Fatalf("归并累加错误: %+v", c)
		}
	}
}
