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

// E.2 P1-A：partial-hour 输入（15:30~16:30）不丢边界小时，完整小时桶全部重建。
func TestProjectUsageRangePartialHourKeepsBoundaryBuckets(t *testing.T) {
	h15 := bucketHour(1750000000)
	h16 := h15 + 3600
	cleanUsageRange(t, h15, h16)

	// 15:35 与 16:25 两条日志。
	require.NoError(t, model.DB.Create(usageTestLog(h15+2100, "gpt-4o", 100)).Error)
	require.NoError(t, model.DB.Create(usageTestLog(h16+1500, "gpt-4o", 100)).Error)

	_, err := ProjectUsageRange(context.Background(), h15+1800, h16+1800) // 15:30~16:30
	require.NoError(t, err)

	rows, err := model.QueryUsageRows(model.UsageProjectionFilter{BucketStart: h15, BucketEnd: h16})
	require.NoError(t, err)
	var count15, count16 int
	for _, r := range rows {
		switch r.BucketTime {
		case h15:
			count15++
		case h16:
			count16++
		}
	}
	require.Equal(t, 1, count15, "15:30 输入应重建 15:00 整点桶且不丢日志")
	require.Equal(t, 1, count16, "16:30 输入应重建 16:00 整点桶且不丢日志")
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
