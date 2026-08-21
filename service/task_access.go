package service

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

// ---------------------------------------------------------------------------
// 单 NewAPI User 下的任务访问边界（V1.1 §10.6 / §10.7）
// ---------------------------------------------------------------------------
//
// 异步任务提交时在 TaskPrivateData.Attribution 持久化了可信归因快照。同一
// NewAPI User 下可能混用多张企业凭证（张三 WorkBuddy / 张三 IDE / 李四
// WorkBuddy）。任务查询阶段必须按当前凭证的快照与任务归因做匹配，而不是仅凭
// user_id 放行，从而在单 User 内细分访问边界。
//
// 无 AttributionSnapshot 的历史任务走 Legacy 策略（§10.7）：AUDIT 沿用原
// user_id 行为但打标记；ENFORCE 由 AI_ATTRIBUTION_LEGACY_TASK_ACCESS=allow|deny
// 控制（默认 allow 仅用于迁移期，最终生产目标 deny）。

// LegacyTaskAccessEnv 是历史无归因任务的访问开关。
const LegacyTaskAccessEnv = "AI_ATTRIBUTION_LEGACY_TASK_ACCESS"

// TaskAccessReason 描述一次任务访问判定结果的原因（供审计/日志）。
type TaskAccessReason string

const (
	TaskAccessAllowedByAttribution TaskAccessReason = "allowed_by_attribution"
	TaskAccessAllowedByPrincipal   TaskAccessReason = "allowed_by_principal_purpose"
	TaskAccessAllowedByApp         TaskAccessReason = "allowed_by_app"
	TaskAccessAllowedByCallerApp   TaskAccessReason = "allowed_by_caller_app"
	TaskAccessAllowedByCaller      TaskAccessReason = "allowed_by_caller"
	TaskAccessAllowedLegacy        TaskAccessReason = "allowed_legacy_task"
	TaskAccessDeniedNoSnapshot     TaskAccessReason = "denied_no_identity_snapshot"
	TaskAccessDeniedNoAttribution  TaskAccessReason = "denied_legacy_task"
	TaskAccessDeniedMode           TaskAccessReason = "denied_identity_mode"
	TaskAccessDeniedTarget         TaskAccessReason = "denied_attribution_target"
	TaskAccessDeniedPrincipal      TaskAccessReason = "denied_principal_purpose"
	TaskAccessDeniedApp            TaskAccessReason = "denied_app"
	TaskAccessDeniedCaller         TaskAccessReason = "denied_caller"
	TaskAccessDeniedCallerApp      TaskAccessReason = "denied_caller_app"
	TaskAccessDeniedDisabled       TaskAccessReason = "denied_profile_disabled"
	TaskAccessDeniedPrincipalDisab TaskAccessReason = "denied_principal_disabled"
	TaskAccessDeniedAppBinding     TaskAccessReason = "denied_app_binding"
)

// legacyTaskAccess 读取 AI_ATTRIBUTION_LEGACY_TASK_ACCESS（§10.7）。
// 返回 (allowed, 是否为显式 deny)。非法值 fail-closed 返回 false。
func legacyTaskAccess() (bool, TaskAccessReason) {
	val := strings.TrimSpace(strings.ToLower(os.Getenv(LegacyTaskAccessEnv)))
	switch val {
	case "allow", "":
		// 默认 allow 仅用于迁移期（§10.7）
		return true, TaskAccessAllowedLegacy
	case "deny":
		return false, TaskAccessDeniedNoAttribution
	default:
		// fail-closed：未知值拒绝，避免误放行历史任务
		return false, TaskAccessDeniedNoAttribution
	}
}

// canAccessBySnapshot 依据当前凭证 IdentitySnapshot 与任务归因快照判定访问权。
// snap 为 nil 时按快照缺失处理。
func canAccessBySnapshot(snap *types.IdentitySnapshot, attr *constant.TrustedAttributionContext) (bool, TaskAccessReason) {
	if snap == nil {
		return false, TaskAccessDeniedNoSnapshot
	}
	if !snap.Enabled {
		return false, TaskAccessDeniedDisabled
	}

	switch attr.IdentityMode {
	case constant.IdentityModeStatic:
		switch attr.AttributionTarget {
		case constant.AttributionTargetPrincipal:
			// §10.6 STATIC/PRINCIPAL：Profile enabled；principal_id 相等；
			// credential_purpose_id 相等。允许“同人+同用途”新 Key 访问旧任务。
			if !snap.PrincipalEnabled {
				return false, TaskAccessDeniedPrincipalDisab
			}
			if snap.PrincipalID != attr.PrincipalID {
				return false, TaskAccessDeniedPrincipal
			}
			if snap.CredentialPurposeID != attr.CredentialPurposeID {
				return false, TaskAccessDeniedPrincipal
			}
			return true, TaskAccessAllowedByPrincipal

		case constant.AttributionTargetApplication:
			// §10.6 STATIC/APPLICATION：当前 Profile 必须仍固定同一个 App。
			if profileHasAppBinding(snap, attr.RootAppID) {
				return true, TaskAccessAllowedByApp
			}
			return false, TaskAccessDeniedApp

		default:
			return false, TaskAccessDeniedTarget
		}

	case constant.IdentityModeDynamic:
		// §10.6 DYNAMIC/PLATFORM：caller_id 等于任务 caller_id，且当前 Profile
		// 对 Root App 仍有 Binding。
		if attr.CallerID == "" || snap.CallerID != attr.CallerID {
			return false, TaskAccessDeniedCaller
		}
		if profileHasAppBinding(snap, attr.RootAppID) {
			return true, TaskAccessAllowedByCaller
		}
		return false, TaskAccessDeniedAppBinding

	case constant.IdentityModeHybrid:
		// §10.6 HYBRID/APPLICATION：Caller 与固定 App 均必须一致。
		if attr.CallerID == "" || snap.CallerID != attr.CallerID {
			return false, TaskAccessDeniedCallerApp
		}
		if profileHasAppBinding(snap, attr.RootAppID) {
			return true, TaskAccessAllowedByCallerApp
		}
		return false, TaskAccessDeniedCallerApp

	default:
		return false, TaskAccessDeniedMode
	}
}

// profileHasAppBinding 判断当前 Profile 对指定 AppCode 是否有有效绑定
// （App 启用且 Binding 启用）。
func profileHasAppBinding(snap *types.IdentitySnapshot, appCode string) bool {
	if appCode == "" {
		return false
	}
	for i := range snap.Applications {
		app := &snap.Applications[i]
		if app.AppCode == appCode && app.AppEnabled && app.BindingEnabled {
			return true
		}
	}
	return false
}

// CanAccessTask 判定当前凭证是否可访问某任务（§10.6/§10.7）。
// snap 为当前请求凭证的 IdentitySnapshot；task 为被查询的任务。
// 归因治理未启用（AI_ATTRIBUTION_MODE=disabled）时恒返回 true，行为与改造前一致。
func CanAccessTask(snap *types.IdentitySnapshot, task *model.Task) (bool, TaskAccessReason) {
	if GetAttributionMode() == constant.AttributionModeDisabled {
		return true, TaskAccessAllowedByAttribution
	}
	attr := task.PrivateData.Attribution
	if attr == nil {
		return legacyTaskAccess()
	}
	return canAccessBySnapshot(snap, attr)
}

// FilterTasksByAttribution 在任务查询结果上应用访问边界，返回仅含当前凭证
// 可访问任务的新切片（保持顺序）。原切片不被修改。
func FilterTasksByAttribution(snap *types.IdentitySnapshot, tasks []*model.Task) []*model.Task {
	if GetAttributionMode() == constant.AttributionModeDisabled {
		return tasks
	}
	filtered := make([]*model.Task, 0, len(tasks))
	for _, t := range tasks {
		if allowed, _ := CanAccessTask(snap, t); allowed {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
