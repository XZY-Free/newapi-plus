package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

// buildAttribution 构造一个 WorkBuddy 弱身份（STATIC/PRINCIPAL）归因上下文。
func buildAttribution(mode, target string) *constant.TrustedAttributionContext {
	return &constant.TrustedAttributionContext{
		TokenID:          101,
		ProfileID:        9,
		IdentityMode:     mode,
		AttributionTarget: target,
		IdentityAssurance: constant.IdentityAssuranceCredentialOnly,
		PrincipalID:       1,
		PrincipalCode:     "zhangsan",
		PrincipalName:     "张三",
		CredentialPurposeID: 10,
		CredentialPurposeCode: "WORKBUDDY",
		UsageTeamID:       55,
		UsageTeamCode:     "teamA",
	}
}

// buildPrincipalSnap 构造对应 Profile 的 IdentitySnapshot。
func buildPrincipalSnap(principalID, purposeID int, enabled bool) *types.IdentitySnapshot {
	return &types.IdentitySnapshot{
		ProfileID:             9,
		TokenID:               101,
		Enabled:               enabled,
		IdentityMode:          constant.IdentityModeStatic,
		AttributionTarget:     constant.AttributionTargetPrincipal,
		PrincipalID:           principalID,
		PrincipalEnabled:      true,
		CredentialPurposeID:   purposeID,
		CredentialPurposeEnabled: true,
	}
}

// enableEnforce 将归因模式切到 enforce，让 CanAccessTask 真正执行匹配。
func enableEnforce(t *testing.T) {
	t.Helper()
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeEnforce)
}

// §10.8-1/2：任务持久化 Principal/Purpose/Usage Team；DB 往返后归因仍存在。
func TestTaskAttributionPersistence(t *testing.T) {
	attr := buildAttribution(constant.IdentityModeStatic, constant.AttributionTargetPrincipal)
	attr.TokenID = 101

	task := &model.Task{PrivateData: model.TaskPrivateData{
		Attribution:  attr,
		TraceContext: &model.TraceContextSnapshot{TraceParent: "00-11111111111111111111111111111111-2222222222222222-01", TraceState: "k=v"},
	}}

	// Value()/Scan() 模拟 gorm type:json 列的写库与重启读回（§10.8-2）。
	val, err := task.PrivateData.Value()
	if err != nil {
		t.Fatalf("TaskPrivateData.Value() error: %v", err)
	}
	var back model.TaskPrivateData
	if err := back.Scan(val); err != nil {
		t.Fatalf("TaskPrivateData.Scan() error: %v", err)
	}
	if back.Attribution == nil {
		t.Fatal("§10.8-2 重启后归因丢失")
	}
	if back.Attribution.PrincipalID != 1 || back.Attribution.CredentialPurposeID != 10 {
		t.Fatalf("§10.8-1 Principal/Purpose 未保存: got %+v", back.Attribution)
	}
	if back.Attribution.UsageTeamID != 55 || back.Attribution.UsageTeamCode != "teamA" {
		t.Fatalf("§10.8-1 Usage Team 未保存: got %+v", back.Attribution)
	}
	if back.TraceContext == nil || back.TraceContext.TraceParent != task.PrivateData.TraceContext.TraceParent {
		t.Fatalf("§10.8-2 TraceContext 未保存: got %+v", back.TraceContext)
	}
}

// §10.8-3：张三新 WorkBuddy Key（同 Principal + Purpose）可访问旧 WorkBuddy 任务。
func TestAccessSamePrincipalPurpose(t *testing.T) {
	enableEnforce(t)
	task := &model.Task{PrivateData: model.TaskPrivateData{
		Attribution: buildAttribution(constant.IdentityModeStatic, constant.AttributionTargetPrincipal),
	}}
	// 新 Key 换了 TokenID，但同 principal_id + 同 purpose_id
	snap := buildPrincipalSnap(1, 10, true)
	snap.TokenID = 202
	allowed, reason := CanAccessTask(snap, task)
	if !allowed {
		t.Fatalf("§10.8-3 同 Principal+Purpose 应放行, got reason=%s", reason)
	}
}

// §10.8-4：张三 IDE Key 不可访问张三 WorkBuddy 任务（purpose 不同）。
func TestAccessDifferentPurpose(t *testing.T) {
	enableEnforce(t)
	task := &model.Task{PrivateData: model.TaskPrivateData{
		Attribution: buildAttribution(constant.IdentityModeStatic, constant.AttributionTargetPrincipal),
	}}
	// 同 principal=1，但 purpose=11 (IDE)
	snap := buildPrincipalSnap(1, 11, true)
	if allowed, reason := CanAccessTask(snap, task); allowed {
		t.Fatalf("§10.8-4 IDE Key 不应访问 WorkBuddy 任务, reason=%s", reason)
	}
}

// §10.8-5：李四 WorkBuddy Key 不可访问张三任务（principal 不同）。
func TestAccessDifferentPrincipal(t *testing.T) {
	enableEnforce(t)
	task := &model.Task{PrivateData: model.TaskPrivateData{
		Attribution: buildAttribution(constant.IdentityModeStatic, constant.AttributionTargetPrincipal),
	}}
	snap := buildPrincipalSnap(2, 10, true) // principal=2 (李四)
	if allowed, reason := CanAccessTask(snap, task); allowed {
		t.Fatalf("§10.8-5 李四不应访问张三任务, reason=%s", reason)
	}
}

// §10.8-6：DYNAMIC caller 相同 + App Binding 合法可访问。
func TestAccessDynamicCallerApp(t *testing.T) {
	enableEnforce(t)
	attr := &constant.TrustedAttributionContext{
		IdentityMode:      constant.IdentityModeDynamic,
		AttributionTarget: constant.AttributionTargetPlatform,
		CallerID:          "caller-9",
		RootAppID:         "app-workbuddy",
	}
	snap := &types.IdentitySnapshot{
		Enabled:      true,
		IdentityMode: constant.IdentityModeDynamic,
		CallerID:     "caller-9",
		Applications: []types.SnapshotApplication{
			{AppCode: "app-workbuddy", AppEnabled: true, BindingEnabled: true},
		},
	}
	if allowed, reason := CanAccessTask(snap, &model.Task{PrivateData: model.TaskPrivateData{Attribution: attr}}); !allowed {
		t.Fatalf("§10.8-6 caller 相同+App Binding 应放行, reason=%s", reason)
	}
}

// §10.8-7：caller 不同即使同一 user_id 也拒绝。
func TestAccessDynamicDifferentCaller(t *testing.T) {
	enableEnforce(t)
	attr := &constant.TrustedAttributionContext{
		IdentityMode:      constant.IdentityModeDynamic,
		AttributionTarget: constant.AttributionTargetPlatform,
		CallerID:          "caller-9",
		RootAppID:         "app-workbuddy",
	}
	snap := &types.IdentitySnapshot{
		Enabled: true,
		CallerID: "caller-OTHER", // 同一 NewAPI user 下另一个 caller
		Applications: []types.SnapshotApplication{
			{AppCode: "app-workbuddy", AppEnabled: true, BindingEnabled: true},
		},
	}
	if allowed, reason := CanAccessTask(snap, &model.Task{PrivateData: model.TaskPrivateData{Attribution: attr}}); allowed {
		t.Fatalf("§10.8-7 caller 不同应拒绝, reason=%s", reason)
	}
}

// §10.8-8：HYBRID App 不同拒绝。
func TestAccessHybridDifferentApp(t *testing.T) {
	enableEnforce(t)
	attr := &constant.TrustedAttributionContext{
		IdentityMode:      constant.IdentityModeHybrid,
		AttributionTarget: constant.AttributionTargetApplication,
		CallerID:          "caller-9",
		RootAppID:         "app-workbuddy",
	}
	snap := &types.IdentitySnapshot{
		Enabled: true,
		CallerID: "caller-9",
		Applications: []types.SnapshotApplication{
			{AppCode: "app-IDE", AppEnabled: true, BindingEnabled: true},
		},
	}
	if allowed, reason := CanAccessTask(snap, &model.Task{PrivateData: model.TaskPrivateData{Attribution: attr}}); allowed {
		t.Fatalf("§10.8-8 HYBRID App 不同应拒绝, reason=%s", reason)
	}
}

// §10.8-9：Refund/Recalculate 日志仍使用提交时归因（taskBillingOther 合并 ai_attribution）。
func TestTaskBillingOtherKeepsAttribution(t *testing.T) {
	attr := buildAttribution(constant.IdentityModeStatic, constant.AttributionTargetPrincipal)
	task := &model.Task{PrivateData: model.TaskPrivateData{Attribution: attr}}
	other := taskBillingOther(task)
	snap, ok := other["ai_attribution"].(map[string]interface{})
	if !ok {
		t.Fatalf("§10.8-9 日志缺少 ai_attribution 快照: %v", other)
	}
	if snap["principal_id"] != 1 || snap["credential_purpose_id"] != 10 || snap["usage_team_id"] != 55 {
		t.Fatalf("§10.8-9 归因字段未保留: %v", snap)
	}
}

// §10.8-11：Legacy allow/deny 行为正确。
func TestLegacyTaskAccess(t *testing.T) {
	enableEnforce(t)
	task := &model.Task{PrivateData: model.TaskPrivateData{}} // 无 AttributionSnapshot
	snap := buildPrincipalSnap(1, 10, true)

	t.Setenv(LegacyTaskAccessEnv, "allow")
	if allowed, _ := CanAccessTask(snap, task); !allowed {
		t.Fatal("§10.8-11 legacy=allow 应放行")
	}

	t.Setenv(LegacyTaskAccessEnv, "deny")
	if allowed, reason := CanAccessTask(snap, task); allowed {
		t.Fatalf("§10.8-11 legacy=deny 应拒绝, reason=%s", reason)
	}
}

// §10.8-11b：归因治理未启用（AI_ATTRIBUTION_MODE 未设）时行为与改造前一致，恒放行。
func TestAccessDisabledModePassesThrough(t *testing.T) {
	t.Setenv(constant.AttributionModeEnv, constant.AttributionModeDisabled)
	task := &model.Task{PrivateData: model.TaskPrivateData{}} // 无归因也放行
	if allowed, _ := CanAccessTask(nil, task); !allowed {
		t.Fatal("disabled 模式应恒放行")
	}
}

// FilterTasksByAttribution 只保留可访问任务，且不修改原切片。
func TestFilterTasksByAttribution(t *testing.T) {
	enableEnforce(t)
	snap := buildPrincipalSnap(1, 10, true)
	tasks := []*model.Task{
		{PrivateData: model.TaskPrivateData{Attribution: buildAttribution(constant.IdentityModeStatic, constant.AttributionTargetPrincipal)}}, // 同人同用途：保留
		{PrivateData: model.TaskPrivateData{Attribution: &constant.TrustedAttributionContext{
			IdentityMode: constant.IdentityModeStatic, AttributionTarget: constant.AttributionTargetPrincipal,
			PrincipalID: 2, CredentialPurposeID: 10,
		}}}, // 李四：过滤
		{PrivateData: model.TaskPrivateData{}}, // legacy allow：保留
	}
	got := FilterTasksByAttribution(snap, tasks)
	if len(got) != 2 {
		t.Fatalf("期望保留 2 条, got %d", len(got))
	}
	if len(tasks) != 3 {
		t.Fatal("FilterTasksByAttribution 不应修改原切片")
	}
}
