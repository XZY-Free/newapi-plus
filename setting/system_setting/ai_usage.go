package system_setting

import "github.com/QuantumNous/new-api/setting/config"

// AIUsageAlertSettings 弱身份确定性异常检测配置（V1.1 §12.5）。
// 所有阈值 0 表示关闭该维度告警；BASELINE_MULTIPLIER 默认 5。
type AIUsageAlertSettings struct {
	HourlyRequestAlert int64   `json:"hourly_request_alert"` // 0 关闭；单位：请求/小时
	HourlyTokenAlert   int64   `json:"hourly_token_alert"`   // 0 关闭；单位：token/小时
	HourlyQuotaAlert   int64   `json:"hourly_quota_alert"`   // 0 关闭；单位：quota/小时
	BaselineMultiplier float64 `json:"baseline_multiplier"`  // 默认 5
	BaselineWindowDays int     `json:"baseline_window_days"` // 最近 N 个有效业务日相同小时，默认 7
}

var defaultAIUsageAlertSettings = AIUsageAlertSettings{
	BaselineMultiplier: 5,
	BaselineWindowDays: 7,
}

func init() {
	config.GlobalConfig.Register("ai_usage_alert", &defaultAIUsageAlertSettings)
}

func GetAIUsageAlertSettings() *AIUsageAlertSettings {
	return &defaultAIUsageAlertSettings
}
