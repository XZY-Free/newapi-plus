package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testToken() *model.Token {
	allowIps := ""
	return &model.Token{
		Id:                 101,
		Name:               "t101",
		AllowIps:           &allowIps,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		RemainQuota:        0,
		ExpiredTime:        -1,
		CreatedTime:        time.Now().Unix(),
	}
}

// 门禁 24：CREDENTIAL_ONLY 且无 IP/Model/Quota/Expiry/RateLimit → HIGH_RISK。
func TestCredentialRiskBottomLineHighRisk(t *testing.T) {
	risk := ComputeCredentialRisk(testToken(), types.ProfileRateLimit{}, true)
	assert.True(t, risk.CredentialOnly)
	assert.False(t, risk.IPRestricted)
	assert.False(t, risk.ModelRestricted)
	assert.False(t, risk.QuotaRestricted)
	assert.False(t, risk.ExpiryConfigured)
	assert.False(t, risk.RateLimitEnabled)
	require.Equal(t, constant.RiskHigh, risk.RiskLevel, "固定底线必须 HIGH_RISK")
}

// 任何一项保护到位则不再触发 HIGH_RISK 底线。
func TestCredentialRiskNotHighRiskWithSingleControl(t *testing.T) {
	// IP 限制到位。
	tk := testToken()
	ip := "1.2.3.4"
	tk.AllowIps = &ip
	risk := ComputeCredentialRisk(tk, types.ProfileRateLimit{}, true)
	assert.NotEqual(t, constant.RiskHigh, risk.RiskLevel)

	// 有配额限制。
	tk2 := testToken()
	tk2.UnlimitedQuota = false
	risk2 := ComputeCredentialRisk(tk2, types.ProfileRateLimit{}, true)
	assert.NotEqual(t, constant.RiskHigh, risk2.RiskLevel)
}

// 强身份（非 CREDENTIAL_ONLY）不受 HIGH_RISK 底线影响。
func TestCredentialRiskStrongIdentityNotHighRisk(t *testing.T) {
	tk := testToken()
	risk := ComputeCredentialRisk(tk, types.ProfileRateLimit{}, false)
	assert.Equal(t, constant.RiskLower, risk.RiskLevel)
}

// 弱身份但保护不足（<2 项控制）→ MEDIUM_RISK。
func TestCredentialRiskMediumWhenUnderProtected(t *testing.T) {
	tk := testToken()
	risk := ComputeCredentialRisk(tk, types.ProfileRateLimit{}, true)
	// 无任何控制 → HIGH（底线）。这里给一项控制：IP。
	ip := "1.2.3.4"
	tk.AllowIps = &ip
	risk = ComputeCredentialRisk(tk, types.ProfileRateLimit{}, true)
	require.Equal(t, constant.RiskMedium, risk.RiskLevel)
}

// 轮换逾期 → MEDIUM_RISK，并计算 overdue days。
// 注意：为避免触发 HIGH_RISK 固定底线，需至少具备一项安全控制。
func TestCredentialRiskRotationOverdue(t *testing.T) {
	tk := testToken()
	ip := "1.2.3.4"
	tk.AllowIps = &ip
	tk.CreatedTime = time.Now().Unix() - 100*86400
	risk := ComputeCredentialRisk(tk, types.ProfileRateLimit{}, true)
	assert.True(t, risk.RotationOverdue)
	require.Equal(t, constant.RiskMedium, risk.RiskLevel)
	assert.GreaterOrEqual(t, risk.RotationOverdueDays, int64(10))
}

// 未逾期则不标记。
func TestCredentialRiskNotOverdue(t *testing.T) {
	tk := testToken()
	tk.CreatedTime = time.Now().Unix() - 10*86400
	risk := ComputeCredentialRisk(tk, types.ProfileRateLimit{}, true)
	assert.False(t, risk.RotationOverdue)
}

// AI_CREDENTIAL_ROTATION_DAYS 范围裁剪 30~365。
func TestCredentialRotationDaysClamped(t *testing.T) {
	t.Setenv(constant.AICredentialRotationDaysEnv, "10")
	require.Equal(t, 30, credentialRotationDays())
	t.Setenv(constant.AICredentialRotationDaysEnv, "999")
	require.Equal(t, 365, credentialRotationDays())
	t.Setenv(constant.AICredentialRotationDaysEnv, "abc")
	require.Equal(t, 90, credentialRotationDays())
	t.Setenv(constant.AICredentialRotationDaysEnv, "120")
	require.Equal(t, 120, credentialRotationDays())
}

// 单一项 IP/Model/Quota/Expiry/RateLimit 标志分别正确。
func TestCredentialRiskFlags(t *testing.T) {
	ip := "1.2.3.4"
	tk := &model.Token{
		AllowIps:           &ip,
		ModelLimitsEnabled: true,
		ModelLimits:        "gpt-4,gpt-4o",
		UnlimitedQuota:     false,
		ExpiredTime:        time.Now().Unix() + 86400,
		CreatedTime:        time.Now().Unix(),
	}
	risk := ComputeCredentialRisk(tk, types.ProfileRateLimit{Enabled: true, WindowSeconds: 60, MaxRequests: 100}, true)
	assert.True(t, risk.IPRestricted)
	assert.True(t, risk.ModelRestricted)
	assert.True(t, risk.QuotaRestricted)
	assert.True(t, risk.ExpiryConfigured)
	assert.True(t, risk.RateLimitEnabled)
	require.Equal(t, constant.RiskLower, risk.RiskLevel)
}
