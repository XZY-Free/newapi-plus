package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// ---------------------------------------------------------------------------
// 弱身份确定性异常检测（V1.1 §12.5）
// ---------------------------------------------------------------------------
//
// 仅记录与展示，不自动禁用 Token；异常必须定位到 profile_id → principal → purpose。
// 不把统计异常提升为“客户端身份验证”。

// UsageAnomaly 一次确定性用量异常提示。
type UsageAnomaly struct {
	ProfileID            int    `json:"profile_id"`
	PrincipalID          int    `json:"principal_id"`
	CredentialPurposeID  int    `json:"credential_purpose_id"`
	BucketTime           int64  `json:"bucket_time"`
	Metric               string `json:"metric"` // request / token / quota
	Current              int64  `json:"current"`
	Baseline             int64  `json:"baseline"`
	Threshold            int64  `json:"threshold"` // max(baseline*multiplier, absolute)
	ModelName            string `json:"model_name"`
	IdentityAssurance    string `json:"identity_assurance"`
}

// UsageAnomalyCandidate 是某维度在某小时的指标快照，供基线比较。
type UsageAnomalyCandidate struct {
	ProfileID           int
	PrincipalID         int
	CredentialPurposeID int
	ModelName           string
	IdentityAssurance   string
	BucketTime          int64
	RequestCount        int64
	TotalTokens         int64
	QuotaNet            int64
}

// median 返回有序切片中位数；空返回 0。
func median(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

// collectCandidateRows 将投影行归并到 (dimension 维度) 候选，同一 profile+purpose+model+hour
// 可能跨多个 caller/app 桶，但异常检测聚焦弱身份 profile→principal→purpose，故按此归并。
func collectCandidateRows(rows []*model.AIUsageHourly) []*UsageAnomalyCandidate {
	candByKey := make(map[string]*UsageAnomalyCandidate)
	for _, r := range rows {
		key := fmt.Sprintf("%d|%d|%d|%s|%s|%d",
			r.ProfileID, r.PrincipalID, r.CredentialPurposeID, r.ModelName, r.IdentityAssurance, r.BucketTime)
		c, ok := candByKey[key]
		if !ok {
			c = &UsageAnomalyCandidate{
				ProfileID: r.ProfileID, PrincipalID: r.PrincipalID, CredentialPurposeID: r.CredentialPurposeID,
				ModelName: r.ModelName, IdentityAssurance: r.IdentityAssurance, BucketTime: r.BucketTime,
			}
			candByKey[key] = c
		}
		c.RequestCount += r.RequestCount
		c.TotalTokens += r.TotalTokens
		c.QuotaNet += r.QuotaNet
	}
	out := make([]*UsageAnomalyCandidate, 0, len(candByKey))
	for _, c := range candByKey {
		out = append(out, c)
	}
	return out
}

// DetectUsageAnomalies 针对 [bucketStart, bucketEnd]（整点）内的投影做确定性异常检测。
// 基线：最近 N 个有效业务日同一小时的中位数；异常 = 当前 >= baseline*multiplier
// 且同时超过绝对最小阈值；历史不足时只使用绝对阈值。
func DetectUsageAnomalies(ctx context.Context, bucketStart, bucketEnd int64) []*UsageAnomaly {
	cfg := system_setting.GetAIUsageAlertSettings()
	multiplier := cfg.BaselineMultiplier
	if multiplier <= 0 {
		multiplier = 5
	}
	days := cfg.BaselineWindowDays
	if days <= 0 {
		days = 7
	}

	var anomalies []*UsageAnomaly
	for bucket := bucketStart; bucket <= bucketEnd; bucket += 3600 {
		currentRows, err := model.QueryUsageRows(model.UsageProjectionFilter{
			BucketStart: bucket, BucketEnd: bucket,
		})
		if err != nil {
			continue
		}
		candidates := collectCandidateRows(currentRows)

		// 读取基线窗口（最近 N 天同一小时）。
		baselineStart := bucket - int64(days)*24*3600
		baselineEnd := bucket - 3600
		baseRows, err := model.QueryUsageRows(model.UsageProjectionFilter{
			BucketStart: baselineStart, BucketEnd: baselineEnd,
		})
		if err != nil {
			baseRows = nil
		}
		baseByKey := collectCandidateRows(baseRows)

		for _, c := range candidates {
			if c.ProfileID == 0 && c.CredentialPurposeID == 0 {
				continue // 未归因桶不参与弱身份异常检测
			}
			// 基线：收集该 profile+purpose 在窗口内同小时的历史值（排除当前桶）。
			var reqHist, tokHist, quotaHist []int64
			for _, b := range baseByKey {
				if b.ProfileID == c.ProfileID && b.CredentialPurposeID == c.CredentialPurposeID &&
					b.ModelName == c.ModelName && hourOfDay(b.BucketTime) == hourOfDay(c.BucketTime) {
					reqHist = append(reqHist, b.RequestCount)
					tokHist = append(tokHist, b.TotalTokens)
					quotaHist = append(quotaHist, b.QuotaNet)
				}
			}
			check := func(metric string, current, baseline int64, absThreshold int64) {
				if absThreshold <= 0 {
					return // 该维度告警关闭
				}
				threshold := int64(float64(baseline) * multiplier)
				if threshold < absThreshold {
					threshold = absThreshold
				}
				if current > 0 && current >= threshold {
					anomalies = append(anomalies, &UsageAnomaly{
						ProfileID: c.ProfileID, PrincipalID: c.PrincipalID,
						CredentialPurposeID: c.CredentialPurposeID, BucketTime: c.BucketTime,
						Metric: metric, Current: current, Baseline: baseline, Threshold: threshold,
						ModelName: c.ModelName, IdentityAssurance: c.IdentityAssurance,
					})
				}
			}
			check("request", c.RequestCount, median(reqHist), cfg.HourlyRequestAlert)
			check("token", c.TotalTokens, median(tokHist), cfg.HourlyTokenAlert)
			check("quota", c.QuotaNet, median(quotaHist), cfg.HourlyQuotaAlert)
		}
	}
	return anomalies
}

// hourOfDay 返回整点 Unix 对应的当天小时序号（0-23），用于“同一小时”基线对齐。
// 注意：跨日/夏令时不在此做复杂日历处理，V1.1 取整点对 24 取模即可解释。
func hourOfDay(bucket int64) int64 {
	return (bucket / 3600) % 24
}
