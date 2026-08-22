package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTaskSelfTest 迁移 Task 表，替换主库为独立内存库。
func setupTaskSelfTest(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
}

// TestGetUserTaskManagementPlaneKeepsAttributedTasks 冻结 P1 #1：/api/task/self 是管理平面
// （UserAuth 会话，无 Profile/Principal/Purpose/Caller/App 上下文）。即使 AI_ATTRIBUTION_MODE
// 为 enforce，管理平面也绝不在 native user_id 之上套用 Credential 数据面的
// FilterTasksByAttribution——否则会把已归因任务错误清空。数据面访问边界由 Credential 数据面
// 各入口（RelayTaskFetch/ResolveOriginTask/videoProxy 等）的 CanAccessTask 承担。
func TestGetUserTaskManagementPlaneKeepsAttributedTasks(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
	setupTaskSelfTest(t)

	// 归因治理启用下的归因任务：管理平面必须仍能看见，不得被 FilterTasksByAttribution 清空。
	require.NoError(t, model.DB.Create(&model.Task{
		UserId: 1,
		TaskID: "attributed-task-1",
		Status: model.TaskStatusNotStart,
		PrivateData: model.TaskPrivateData{
			Attribution: &constant.TrustedAttributionContext{
				TokenID: 101, ProfileID: 9, IdentityMode: constant.IdentityModeStatic,
				AttributionTarget: constant.AttributionTargetPrincipal,
				PrincipalID:       1, CredentialPurposeID: 10,
			},
		},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/task/self?page=1&page_size=10", nil)
	c.Set("id", 1) // 管理平面：native user id，无 token_id / Profile 上下文

	GetUserTask(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Items []map[string]interface{} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data.Items, 1, "管理平面必须返回归因任务，不得被 FilterTasksByAttribution 清空")
}
