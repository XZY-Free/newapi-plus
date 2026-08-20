package service

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

// ComputeCredentialRisk 计算弱身份凭证安全姿态（文档 6.15.2）。
//
// 仅读取现有 NewAPI Token 的 AllowIps / ModelLimits / RemainQuota / UnlimitedQuota /
// ExpiredTime / CreatedTime，以及 Profile 级限流配置。绝不读取 Token Key。
//
// 固定底线：CREDENTIAL_ONLY 且无 IP、无模型限制、无限额度、无过期、无 Profile 级限流
// → HIGH_RISK。
func ComputeCredentialRisk(token *model.Token, rateLimit types.ProfileRateLimit, credentialOnly bool) types.RiskPosture {
	p := types.RiskPosture{
		CredentialOnly:   credentialOnly,
		RateLimitEnabled: rateLimit.Enabled,
	}
	if token == nil {
		p.RiskLevel = constant.RiskHigh
		return p
	}

	// IP 限制：采用真实 IP 解析语义（GetIpLimits 处理逗号/空白/换行），
	// 解析结果非空才算受限。
	p.IPRestricted = len(token.GetIpLimits()) > 0

	// Model 限制：启用且模型列表非空。
	p.ModelRestricted = token.ModelLimitsEnabled && strings.TrimSpace(token.ModelLimits) != ""
	p.QuotaRestricted = !token.UnlimitedQuota
	// ExpiredTime == -1 表示永不过期；否则一律视为已配置有效期（兼容 0 与已过期时间）。
	p.ExpiryConfigured = token.ExpiredTime != -1

	rotationDays := credentialRotationDays()
	if token.CreatedTime > 0 && rotationDays > 0 {
		now := time.Now().Unix()
		if now > token.CreatedTime {
			ageDays := (now - token.CreatedTime) / 86400
			if ageDays > int64(rotationDays) {
				p.RotationOverdue = true
				p.RotationOverdueDays = ageDays - int64(rotationDays)
			}
		}
	}

	p.RiskLevel = classifyRisk(&p)
	return p
}

// classifyRisk 依据安全姿态标志计算风险等级。
func classifyRisk(p *types.RiskPosture) string {
	controls := 0
	if p.IPRestricted {
		controls++
	}
	if p.ModelRestricted {
		controls++
	}
	if p.QuotaRestricted {
		controls++
	}
	if p.ExpiryConfigured {
		controls++
	}
	if p.RateLimitEnabled {
		controls++
	}

	// 固定底线：CREDENTIAL_ONLY 且所有控制项全部缺失。
	if p.CredentialOnly && controls == 0 {
		return constant.RiskHigh
	}
	// 轮换逾期升级为 MEDIUM（即使强身份）。
	if p.RotationOverdue {
		return constant.RiskMedium
	}
	// 弱身份但保护不足（少于两项控制）→ MEDIUM。
	if p.CredentialOnly && controls < 2 {
		return constant.RiskMedium
	}
	return constant.RiskLower
}

// credentialRotationDays 读取 AI_CREDENTIAL_ROTATION_DAYS，默认 90，范围 30~365。
// 解析失败时使用默认值。
func credentialRotationDays() int {
	days := constant.AICredentialRotationDaysDefault
	raw := os.Getenv(constant.AICredentialRotationDaysEnv)
	if raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			days = v
		} else {
			common.SysError("failed to parse " + constant.AICredentialRotationDaysEnv + ", using default 90")
		}
	}
	if days < constant.AICredentialRotationDaysMin {
		days = constant.AICredentialRotationDaysMin
	}
	if days > constant.AICredentialRotationDaysMax {
		days = constant.AICredentialRotationDaysMax
	}
	return days
}
