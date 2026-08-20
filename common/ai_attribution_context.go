package common

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// 第一批只提供 Trusted Attribution 在 Gin Context 中的 Set/Get 基础，
// 不挂载运行时中间件。第二批身份中间件将使用这些函数写入可信归因上下文。
func SetTrustedAttribution(c *gin.Context, ctx *constant.TrustedAttributionContext) {
	if ctx == nil {
		return
	}
	SetContextKey(c, constant.ContextKeyTrustedAttribution, ctx)
}

// GetTrustedAttribution 从 Gin Context 读取可信归因上下文。
// 返回的 bool 表示是否已设置且类型正确。
func GetTrustedAttribution(c *gin.Context) (*constant.TrustedAttributionContext, bool) {
	value, ok := c.Get(string(constant.ContextKeyTrustedAttribution))
	if !ok {
		return nil, false
	}
	ctx, ok := value.(*constant.TrustedAttributionContext)
	return ctx, ok
}
