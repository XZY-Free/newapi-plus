/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// usageTestLog 构造一条带归因快照的模型请求日志（非 Task Billing 调整）。
func usageTestLog(ts int64, modelName string, quota int) *model.Log {
	attr := map[string]interface{}{"profile_id": 9, "identity_assurance": "CREDENTIAL_ONLY", "client_verified": false}
	return &model.Log{
		CreatedAt: ts, ModelName: modelName, Type: model.LogTypeConsume,
		PromptTokens: 10, CompletionTokens: 5, Quota: quota,
		Other: common.MapToJsonStr(map[string]interface{}{"ai_attribution": attr}),
	}
}

// cleanUsageRange 清空某时间范围内的 Log 与投影行，避免测试间日志泄漏导致重复累计。
// 测试共享同一 sqlite DB（service.TestMain），不同测试的读取区间若重叠会互相污染。
func cleanUsageRange(t *testing.T, start, end int64) {
	t.Helper()
	require.NoError(t, model.DB.Where("created_at >= ? AND created_at <= ?", start, end).Delete(&model.Log{}).Error)
	require.NoError(t, model.DB.Where("bucket_time >= ? AND bucket_time <= ?", start, end).Delete(&model.AIUsageHourly{}).Error)
}

// E.2 P1-A residual：partial-hour 输入（15:30~16:30）必须按完整小时读取 Log，
// 边界桶不丢。日志落在 15:10/15:35（15:00 桶）与 16:25/16:50（16:00 桶），其中
// 15:10 在输入起点 15:30 之前、16:50 在输入终点 16:30 之后，仍须全部进入整点桶。
// 同时验证空 Other 日志被正常 skip（不视为解析失败、不中断 rebuild）。
func TestProjectUsageRangePartialHourKeepsBoundaryBuckets(t *testing.T) {
	h15 := bucketHour(1750000000)
	h16 := h15 + 3600
	cleanUsageRange(t, h15, h16)

	// 15:10（早于输入起点）、15:35 → 15:00 桶；16:25、16:50（晚于输入终点）→ 16:00 桶。
	require.NoError(t, model.DB.Create(usageTestLog(h15+600, "gpt-4o", 100)).Error)  // 15:10
	require.NoError(t, model.DB.Create(usageTestLog(h15+2100, "gpt-4o", 100)).Error) // 15:35
	require.NoError(t, model.DB.Create(usageTestLog(h16+1500, "gpt-4o", 100)).Error) // 16:25
	require.NoError(t, model.DB.Create(usageTestLog(h16+3000, "gpt-4o", 100)).Error) // 16:50
	// 空 Other 日志：应被正常 skip，rebuild 仍成功，不计数。
	empty := usageTestLog(h15+1200, "gpt-4o", 50)
	empty.Other = ""
	require.NoError(t, model.DB.Create(empty).Error)

	_, err := ProjectUsageRange(context.Background(), h15+1800, h16+1800) // 15:30~16:30
	require.NoError(t, err)

	rows, err := model.QueryUsageRows(model.UsageProjectionFilter{BucketStart: h15, BucketEnd: h16})
	require.NoError(t, err)
	var count15, count16 int
	var req15, req16 int64
	for _, r := range rows {
		switch r.BucketTime {
		case h15:
			count15++
			req15 += r.RequestCount
		case h16:
			count16++
			req16 += r.RequestCount
		}
	}
	require.Equal(t, 1, count15, "15:00 整点桶应聚合 15:10+15:35 两条日志")
	require.Equal(t, int64(2), req15, "15:00 桶请求数应为 2")
	require.Equal(t, 1, count16, "16:00 整点桶应聚合 16:25+16:50 两条日志")
	require.Equal(t, int64(2), req16, "16:00 桶请求数应为 2")
}

// E.2 P1-A residual：malformed Other JSON 属 rebuild parsing failure，返回 error，
// 不执行 Replace，旧 projection 完整保留。
func TestProjectUsageRangeMalformedOtherKeepsOldProjection(t *testing.T) {
	h := bucketHour(1750002000)
	require.NoError(t, model.DB.Where("bucket_time = ?", h).Delete(&model.AIUsageHourly{}).Error)
	old := &model.AIUsageHourly{
		BucketTime: h, ProfileID: 9, IdentityAssurance: "CREDENTIAL_ONLY",
		DimensionHash: "old-dim-other", RequestCount: 7, ModelName: "gpt-4o",
	}
	require.NoError(t, model.DB.Create(old).Error)

	// 一条真正 JSON parse error 的日志。
	malformed := &model.Log{
		CreatedAt: h + 300, ModelName: "gpt-4o", Type: model.LogTypeConsume,
		PromptTokens: 1, CompletionTokens: 1, Quota: 10, Other: "{not valid json",
	}
	require.NoError(t, model.DB.Create(malformed).Error)

	_, err := ProjectUsageRange(context.Background(), h, h+3600)
	require.Error(t, err, "malformed Other 应视为 rebuild parsing failure")

	var cnt int64
	require.NoError(t, model.DB.Model(&model.AIUsageHourly{}).Where("bucket_time = ?", h).Count(&cnt).Error)
	require.Equal(t, int64(1), cnt, "解析失败时不得执行 Replace，旧 projection 必须完整保留")
	var got model.AIUsageHourly
	require.NoError(t, model.DB.Where("bucket_time = ?", h).First(&got).Error)
	require.Equal(t, int64(7), got.RequestCount)
}

// E.2 P1-A：相同范围重复 rebuild 结果幂等（不因重复累加而翻倍）。
func TestProjectUsageRangeRepeatIdempotent(t *testing.T) {
	h := bucketHour(1750000000)
	cleanUsageRange(t, h, h+3600)
	require.NoError(t, model.DB.Create(usageTestLog(h+600, "gpt-4o", 100)).Error)

	_, err := ProjectUsageRange(context.Background(), h, h+3600)
	require.NoError(t, err)
	_, err = ProjectUsageRange(context.Background(), h, h+3600)
	require.NoError(t, err)

	rows, err := model.QueryUsageRows(model.UsageProjectionFilter{BucketStart: h, BucketEnd: h})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(1), rows[0].RequestCount, "重复 rebuild 不得重复累计请求")
}

// E.2 P1-A：重建在读取/计算完成前失败时，旧 projection 完整保留（不得先删旧数据）。
func TestProjectUsageRangeCancelledKeepsOldProjection(t *testing.T) {
	h := bucketHour(1749996000)
	require.NoError(t, model.DB.Where("bucket_time = ?", h).Delete(&model.AIUsageHourly{}).Error)
	old := &model.AIUsageHourly{
		BucketTime: h, ProfileID: 9, IdentityAssurance: "CREDENTIAL_ONLY",
		DimensionHash: "old-dim", RequestCount: 7, ModelName: "gpt-4o",
	}
	require.NoError(t, model.DB.Create(old).Error)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ProjectUsageRange(ctx, h, h+3600)
	require.Error(t, err, "取消的 context 应在读取前返回错误")

	var cnt int64
	require.NoError(t, model.DB.Model(&model.AIUsageHourly{}).Where("bucket_time = ?", h).Count(&cnt).Error)
	require.Equal(t, int64(1), cnt, "重建失败时旧 projection 必须完整保留")
	var got model.AIUsageHourly
	require.NoError(t, model.DB.Where("bucket_time = ?", h).First(&got).Error)
	require.Equal(t, int64(7), got.RequestCount)
}

// E.2 P1-E：QueryUsageRowsPage 真实服务端分页（LIMIT/OFFSET + 总数 + 稳定排序）。
func TestQueryUsageRowsPageServerPagination(t *testing.T) {
	h := bucketHour(1750000000)
	require.NoError(t, model.DB.Where("bucket_time >= ? AND bucket_time <= ?", h, h+7200).Delete(&model.AIUsageHourly{}).Error)
	// 造 3 行，跨 3 个桶，便于验证排序与总数。
	for i := 0; i < 3; i++ {
		row := &model.AIUsageHourly{
			BucketTime: h + int64(i*3600), ProfileID: 9, IdentityAssurance: "CREDENTIAL_ONLY",
			DimensionHash: fmt.Sprintf("dim-%d-%d", i, h), RequestCount: 1, ModelName: "gpt-4o",
		}
		require.NoError(t, model.DB.Create(row).Error)
	}

	rows, total, err := model.QueryUsageRowsPage(model.UsageProjectionFilter{BucketStart: h, BucketEnd: h + 7200}, 1, 2)
	require.NoError(t, err)
	require.Equal(t, int64(3), total, "total 应为全量行数")
	require.Len(t, rows, 2, "第一页应返回 pageSize 条")
	// 排序：bucket_time DESC。
	if len(rows) == 2 {
		require.Greater(t, rows[0].BucketTime, rows[1].BucketTime, "应按 bucket_time DESC 排序")
	}

	rows2, total2, err := model.QueryUsageRowsPage(model.UsageProjectionFilter{BucketStart: h, BucketEnd: h + 7200}, 2, 2)
	require.NoError(t, err)
	require.Equal(t, int64(3), total2)
	require.Len(t, rows2, 1, "第二页应返回剩余 1 条")
}

// E.2 P1-D：非整点 range 仍命中整点 bucket 并检出弱身份异常。
func TestDetectUsageAnomaliesNonWholeHourAlignsToBucket(t *testing.T) {
	cfg := system_setting.GetAIUsageAlertSettings()
	cfg.HourlyRequestAlert = 10
	cfg.HourlyTokenAlert = 0
	cfg.HourlyQuotaAlert = 0
	defer func() {
		cfg.HourlyRequestAlert = 0
	}()

	h := bucketHour(1750000000)
	require.NoError(t, model.DB.Where("bucket_time = ?", h).Delete(&model.AIUsageHourly{}).Error)
	require.NoError(t, model.DB.Create(&model.AIUsageHourly{
		BucketTime: h, ProfileID: 9, PrincipalID: 1, CredentialPurposeID: 10,
		ModelName: "gpt-4o", IdentityAssurance: "CREDENTIAL_ONLY",
		DimensionHash: "weak-high", RequestCount: 50,
	}).Error)

	anomalies, err := DetectUsageAnomalies(context.Background(), h+1200, h+2400)
	require.NoError(t, err)
	found := false
	for _, a := range anomalies {
		if a.Metric == "request" && a.BucketTime == h && a.ProfileID == 9 {
			found = true
		}
	}
	require.True(t, found, "非整点 range 应规范化到整点 bucket 并检出 request 异常")
}

// E.2 P1-D：DYNAMIC/HYBRID 强身份 Profile 不产生“弱身份凭证异常”。
func TestDetectUsageAnomaliesStrongProfileNoWeakAnomaly(t *testing.T) {
	cfg := system_setting.GetAIUsageAlertSettings()
	cfg.HourlyRequestAlert = 10
	cfg.HourlyTokenAlert = 0
	cfg.HourlyQuotaAlert = 0
	defer func() {
		cfg.HourlyRequestAlert = 0
	}()

	h := bucketHour(1750002000)
	require.NoError(t, model.DB.Where("bucket_time = ?", h).Delete(&model.AIUsageHourly{}).Error)
	require.NoError(t, model.DB.Create(&model.AIUsageHourly{
		BucketTime: h, ProfileID: 9, PrincipalID: 1, CredentialPurposeID: 10,
		ModelName: "gpt-4o", IdentityAssurance: "HYBRID_VERIFIED_CONTEXT",
		DimensionHash: "strong-high", RequestCount: 500,
	}).Error)

	anomalies, err := DetectUsageAnomalies(context.Background(), h, h)
	require.NoError(t, err)
	for _, a := range anomalies {
		require.NotEqual(t, "request", a.Metric, "强身份 Profile 不应产生弱身份凭证异常")
	}
}

// E.2 P1-D：DB 查询失败必须返回 error，绝不伪装成 200 + 空数组。
func TestDetectUsageAnomaliesDBFailureReturnsError(t *testing.T) {
	orig := model.DB
	bad, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = bad
	defer func() { model.DB = orig }()

	_, err = DetectUsageAnomalies(context.Background(), 1749996000, 1750003200)
	require.Error(t, err, "缺失 ai_usage_hourly 表时应返回 error 而非静默空结果")
}

// E.2 P1-D residual：每个候选的基线必须来自其自己之前的 N 天滚动窗口，绝不混入候选
// 自身或当前范围中晚于它的小时（未来数据）。当前范围 [D, D+7 天] 内，D 的候选当前
// =500，历史 D-7..D-1 同小时 =100，未来 D+1..D+7 同小时 =1000。
//   - 正确（滚动窗口过滤）：基线 = median(100×7) = 100 → threshold=500 → 500 触发异常。
//   - 旧实现（不按候选窗口过滤）：基线混入 D 自身(500)与未来(1000×7)，median=500 →
//     threshold=2500 → 500 不触发。测试将失败，从而捕获回归。
func TestDetectUsageAnomaliesCandidateBaselineRollingWindow(t *testing.T) {
	cfg := system_setting.GetAIUsageAlertSettings()
	cfg.HourlyRequestAlert = 100
	cfg.BaselineMultiplier = 5
	cfg.BaselineWindowDays = 7
	cfg.HourlyTokenAlert = 0
	cfg.HourlyQuotaAlert = 0
	defer func() {
		cfg.HourlyRequestAlert = 0
		cfg.BaselineMultiplier = 0
		cfg.BaselineWindowDays = 0
	}()

	h := bucketHour(1750000000) // 候选日 D，整点小时
	day := int64(24 * 3600)
	// 清理本测试覆盖的整段范围（D-8 天 ~ D+8 天），避免与其他测试共享 DB 泄漏。
	require.NoError(t, model.DB.Where("bucket_time >= ? AND bucket_time <= ?",
		h-8*day, h+8*day).Delete(&model.AIUsageHourly{}).Error)

	mk := func(offsetDay int64, req int64) *model.AIUsageHourly {
		return &model.AIUsageHourly{
			BucketTime: h + offsetDay*day, ProfileID: 9, PrincipalID: 1, CredentialPurposeID: 10,
			ModelName: "gpt-4o", IdentityAssurance: "CREDENTIAL_ONLY",
			DimensionHash: fmt.Sprintf("dim-rw-%d-%d", offsetDay, req),
			RequestCount:  req,
		}
	}
	// 真实历史 D-7 .. D-1 同小时 = 100。
	for d := int64(-7); d <= -1; d++ {
		require.NoError(t, model.DB.Create(mk(d, 100)).Error)
	}
	// 未来 D+1 .. D+7 同小时 = 1000（位于当前范围内，必须被滚动窗口排除在 D 的基线外）。
	for d := int64(1); d <= 7; d++ {
		require.NoError(t, model.DB.Create(mk(d, 1000)).Error)
	}
	// 候选：D 当前 = 500。
	require.NoError(t, model.DB.Create(mk(0, 500)).Error)

	anomalies, err := DetectUsageAnomalies(context.Background(), h, h+7*day)
	require.NoError(t, err)

	var found bool
	for _, a := range anomalies {
		if a.Metric == "request" && a.BucketTime == h && a.ProfileID == 9 {
			found = true
			require.Equal(t, int64(100), a.Baseline, "基线只能来自 D-7..D-1 的历史 100，不得混入 D 自身或未来 1000")
			require.Equal(t, int64(500), a.Threshold, "threshold = max(baseline*5, abs100) = 500")
			require.Equal(t, int64(500), a.Current)
		}
	}
	require.True(t, found, "D 当前 500 相对历史基线 100（*5=500）应产生 request 异常；若混入未来 1000 将不触发")
}
