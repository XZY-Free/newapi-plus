package common

// 第三批“消费事实归因与可查询日志”RelayInfo 扩展回归测试（冻结方案 V1.1 8.2 / 8.10）。
// 契约：RelayInfo 必须新增唯一的公开归因快照字段 Attribution（*TrustedAttributionContext 兼容），
// genBaseRelayInfo / GenRelayInfo 从 Gin Context 克隆 Trusted Context 到该字段；
// 后续修改原 Trusted Context 不得影响 RelayInfo.Attribution。

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestGenRelayInfoClonesAttributionFromGinContext
// 要求：GenRelayInfo 后，info.Attribution 从 Gin Context 克隆，且为独立对象。
func TestGenRelayInfoClonesAttributionFromGinContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	trusted := &constant.TrustedAttributionContext{
		TokenID:            999,
		ProfileID:          888,
		CredentialVerified: true,
		Environment:        "prod",
		IdentityMode:       constant.IdentityModeStatic,
		AttributionTarget:  constant.AttributionTargetPrincipal,
		IdentityAssurance:  constant.IdentityAssuranceCredentialOnly,
		IdentitySource:     constant.IdentitySourceToken,
		IdentityVerified:   true,
		PrincipalID:        300,
		PrincipalName:      "张三",
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetTrustedAttribution(ctx, trusted)

	info := genBaseRelayInfo(ctx, nil)
	require.NotNil(t, info, "genBaseRelayInfo must return a RelayInfo")
	require.NotNil(t, info.Attribution, "RelayInfo.Attribution must be populated from Gin Context")

	// 克隆的信任快照内容一致。
	require.Equal(t, trusted.TokenID, info.Attribution.TokenID)
	require.Equal(t, trusted.PrincipalName, info.Attribution.PrincipalName)
	require.Equal(t, trusted.CredentialVerified, info.Attribution.CredentialVerified)

	// 修改原 Trusted Context 不影响已克隆的 Attribution（独立深拷贝）。
	trusted.PrincipalName = "李四"
	trusted.TokenID = 111
	require.Equal(t, "张三", info.Attribution.PrincipalName)
	require.Equal(t, 999, info.Attribution.TokenID)
}

// TestGenRelayInfoWithoutTrustedContextLeavesAttributionNil
// 无 Trusted Context 时 Attribution 应为 nil，不得凭空生成。
func TestGenRelayInfoWithoutTrustedContextLeavesAttributionNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := genBaseRelayInfo(ctx, nil)
	require.NotNil(t, info, "genBaseRelayInfo must return a RelayInfo")
	require.Nil(t, info.Attribution, "Attribution must be nil without Trusted Context")
}
