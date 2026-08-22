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
// other 是日志的完整 Other map（含 ai_attribution 与 task 计费标记）。
//
// 区分两类日志：
//   - 模型请求事实（Consume/Error，无 task_id）：request_count=1，success/error 正常计数。
//   - 异步 Task Billing 调整（补扣/退款/重算，带 task_id）：request/success/error 均 +0，
//     Quota 为净变化（Consume→+quota，Refund→-quota），不把退款正数再累计到 quota_net。
//
// 身份规范化（P1-B）与时长单位换算（UseTime=秒 → ms，P1-F）在此统一处理。
func BuildUsageProjectionRow(log *model.Log, other map[string]interface{}, bucketTime int64) (*model.AIUsageHourly, bool) {
	if other == nil {
		return nil, false
	}
	a, _ := other["ai_attribution"].(map[string]interface{})
	dim, hasAttr := model.ParseUsageAttribution(a)
	if !hasAttr {
		return nil, false
	}
	dim.NormalizeStrongUnverified() // P1-B：强身份未验证流量清空 Caller/App 维度
	dim.BucketTime = bucketTime
	dim.ModelName = log.ModelName
	dim.AppID = model.ResolveAppIDFromCode(dim.RootAppCode)

	row := model.AIUsageHourlyRow(dim)
	row.InputTokens = int64(log.PromptTokens)
	row.OutputTokens = int64(log.CompletionTokens)
	row.TotalTokens = int64(log.PromptTokens + log.CompletionTokens)
	row.DurationMsTotal = int64(log.UseTime) * 1000 // P1-F：NewAPI Log.UseTime 单位为秒

	if isTaskBillingAdjustment(other) {
		// P1-C：异步计费调整不制造额外模型请求，仅记录净 Quota 变化。
		row.RequestCount = 0
		row.SuccessCount = 0
		row.ErrorCount = 0
		if log.Type == model.LogTypeRefund {
			row.QuotaNet = -int64(log.Quota) // 退款 → 负净额
		} else {
			row.QuotaNet = int64(log.Quota) // 补扣 → 正净额
		}
		return &row, true
	}

	row.RequestCount = 1
	row.SuccessCount = 0
	row.ErrorCount = 0
	if log.Type == model.LogTypeError {
		row.ErrorCount = 1
	} else {
		row.SuccessCount = 1
	}
	row.QuotaNet = int64(log.Quota)
	return &row, true
}

// isTaskBillingAdjustment 判断日志是否为异步 Task Billing 调整。
// 原始任务提交用 is_task=true（无 task_id），而 Refund/Recalculate 均带 task_id（E.2 P1-C）。
func isTaskBillingAdjustment(other map[string]interface{}) bool {
	taskID, _ := other["task_id"].(string)
	return taskID != ""
}

// bucketHour 将 Unix 秒归一到整点。
func bucketHour(ts int64) int64 {
	return (ts / 3600) * 3600
}

// ProjectUsageRange 将 [start, end]（Unix 秒）内 Log 归一到整点投影，并原子替换该范围。
// 幂等；失败不留下半清空数据（§12.6 / E.2 P1-A）。
//
// 输入先规范化为完整小时 bucketStart/bucketEnd（粒度固定为整点小时），Log 读取范围
// 必须覆盖这些完整小时：logReadStart = bucketStart、logReadEnd = bucketEnd + 3599
// （GetLogsByTimeRange 用 created_at <= end 闭区间）。否则输入 15:30~16:30 会丢失
// 15:00~15:29 与 16:31~16:59 的日志。绝不用原始 start/end 读取。
//
// 全部 Log 解析/聚合成功后经 ReplaceUsageProjectionRange 在单事务内原子替换。绝不在
// 读取/计算完成前删除旧投影：任何读取、Context 或数据库错误发生在 Replace 之前时，原
// 投影完整保留。malformed Other JSON 属 rebuild parsing failure，直接返回 error，
// 不执行 Replace、原投影完整保留；Other 为空或合法 JSON 但无 ai_attribution 正常 skip。
func ProjectUsageRange(ctx context.Context, start, end int64) (int, error) {
	if end <= start {
		return 0, nil
	}
	bucketStart := bucketHour(start)
	bucketEnd := bucketHour(end)
	logReadStart := bucketStart
	logReadEnd := bucketEnd + 3599 // 闭区间覆盖最后一个完整小时的最后一秒

	rows := make([]*model.AIUsageHourly, 0, 256)
	seen := make(map[string]*model.AIUsageHourly) // dimension_hash -> row，先累加再一次性写回
	offset := 0
	total := 0
	for {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}
		logs, err := model.GetLogsByTimeRange(logReadStart, logReadEnd, usageProjectionPageSize, offset)
		if err != nil {
			return total, err
		}
		if len(logs) == 0 {
			break
		}
		for _, log := range logs {
			other, err := parseLogOther(log.Other)
			if err != nil {
				// malformed Other JSON → rebuild parsing failure：保留旧投影，不执行 Replace。
				return total, fmt.Errorf("parse log other failed (id=%d): %w", log.Id, err)
			}
			if other == nil {
				continue // 空 Other 正常 skip
			}
			row, ok := BuildUsageProjectionRow(log, other, bucketHour(log.CreatedAt))
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

// QueryUsageStats 按筛选条件分页聚合企业用量（§12.7 / E.2 P1-E），返回一页明细与总数。
func QueryUsageStats(f model.UsageProjectionFilter, page, pageSize int) ([]*model.AIUsageHourly, int64, error) {
	return model.QueryUsageRowsPage(f, page, pageSize)
}
