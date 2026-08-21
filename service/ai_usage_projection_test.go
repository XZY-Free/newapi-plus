package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

// §12.6：BuildUsageProjectionRow 将单条日志归一到整点投影行（成功计数）。
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
	row, ok := BuildUsageProjectionRow(log, attribution, bucketHour(log.CreatedAt))
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
	if row.QuotaNet != 3000 || row.DurationMsTotal != 1200 {
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
	row, ok := BuildUsageProjectionRow(log, attribution, bucketHour(log.CreatedAt))
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
