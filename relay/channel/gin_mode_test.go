package channel

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMain 在包级一次性进入 gin TestMode。多个 t.Parallel() 的 HeaderOverride 测试
// 若各自调用 gin.SetMode 会对 gin 全局模式变量产生 data race（-race 下报错），
// 因此统一在此设置，测试内不再重复调用。
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
