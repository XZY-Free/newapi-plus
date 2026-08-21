package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// ---------------------------------------------------------------------------
// 企业用量投影（V1.1 §12）
// ---------------------------------------------------------------------------
//
// 事实来源仍是 Log 的 Other.ai_attribution 快照；ai_usage_hourly 只是查询优化。
// 重建在 Go 中解析 Log.Other.ai_attribution，绝不依赖数据库特定 JSON SQL（§12.6）。

const usageProjectionPageSize = 1000

// BuildUsageProjectionRow 由单条日志归一到一行整点投影（纯函数，可测）。
// logType 用于判定成功/错误：LogTypeError 计为 error，其余计入 success（Consume/TaskBilling）。
func BuildUsageProjectionRow(log *model.Log, a map[string]interface{}, bucketTime int64) (*model.AIUsageHourly, bool) {
	dim, hasAttr := model.ParseUsageAttribution(a)
	if !hasAttr {
		return nil, false
	}
	dim.BucketTime = bucketTime
	dim.ModelName = log.ModelName
	dim.AppID = model.ResolveAppIDFromCode(dim.RootAppCode)

	row := model.AIUsageHourlyRow(dim)
	row.RequestCount = 1
	row.SuccessCount = 0
	row.ErrorCount = 0
	if log.Type == model.LogTypeError {
		row.ErrorCount = 1
	} else {
		row.SuccessCount = 1
	}
	row.InputTokens = int64(log.PromptTokens)
	row.OutputTokens = int64(log.CompletionTokens)
	row.TotalTokens = int64(log.PromptTokens + log.CompletionTokens)
	row.QuotaNet = int64(log.Quota)
	row.DurationMsTotal = int64(log.UseTime)
	return &row, true
}

// bucketHour 将 Unix 秒归一到整点。
func bucketHour(ts int64) int64 {
	return (ts / 3600) * 3600
}

// ProjectUsageRange 将 [start, end]（Unix 秒）内 Log 归一到整点投影，并原子替换该范围。
// 幂等；失败不留下半清空数据（§12.6）。
func ProjectUsageRange(ctx context.Context, start, end int64) (int, error) {
	if end <= start {
		return 0, nil
	}
	bucketStart := bucketHour(start)
	bucketEnd := bucketHour(end)
	// 先清空目标范围，避免旧数据混入（§12.6 原子替换语义）。
	if err := model.DeleteUsageProjectionRange(bucketStart, bucketEnd); err != nil {
		return 0, err
	}

	rows := make([]*model.AIUsageHourly, 0, 256)
	seen := make(map[string]*model.AIUsageHourly) // dimension_hash -> row，先累加再一次性写回
	offset := 0
	total := 0
	for {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}
		logs, err := model.GetLogsByTimeRange(start, end, usageProjectionPageSize, offset)
		if err != nil {
			return total, err
		}
		if len(logs) == 0 {
			break
		}
		for _, log := range logs {
			other, err := parseLogOther(log.Other)
			if err != nil || other == nil {
				continue
			}
			a, _ := other["ai_attribution"].(map[string]interface{})
			if a == nil {
				continue
			}
			row, ok := BuildUsageProjectionRow(log, a, bucketHour(log.CreatedAt))
			if !ok {
				continue
			}
			if existing, ok := seen[row.DimensionHash]; ok {
				accumulateUsageRow(existing, row)
			} else {
				seen[row.DimensionHash] = row
				rows = append(rows, row)
			}
			total++
		}
		if len(logs) < usageProjectionPageSize {
			break
		}
		offset += usageProjectionPageSize
	}

	if err := model.ReplaceUsageProjectionRange(bucketStart, bucketEnd, rows); err != nil {
		return total, err
	}
	logger.LogInfo(ctx, fmt.Sprintf("用量投影重建完成：%d 条日志 → %d 个维度桶", total, len(rows)))
	return total, nil
}

// accumulateUsageRow 将 src 累加到 dst（不重复计数）。
func accumulateUsageRow(dst, src *model.AIUsageHourly) {
	dst.RequestCount += src.RequestCount
	dst.SuccessCount += src.SuccessCount
	dst.ErrorCount += src.ErrorCount
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.TotalTokens += src.TotalTokens
	dst.QuotaNet += src.QuotaNet
	dst.DurationMsTotal += src.DurationMsTotal
}

// parseLogOther 将 Log.Other JSON 字符串解析为 map。
func parseLogOther(other string) (map[string]interface{}, error) {
	if other == "" {
		return nil, nil
	}
	m, err := common.StrToMap(other)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// QueryUsageStats 按筛选条件聚合企业用量（§12.7），返回投影明细行。
func QueryUsageStats(f model.UsageProjectionFilter) ([]*model.AIUsageHourly, error) {
	return model.QueryUsageRows(f)
}
