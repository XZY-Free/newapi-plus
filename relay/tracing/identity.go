package tracing

import (
	"context"
	"sync"

	"github.com/QuantumNous/new-api/constant"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// 企业 Span 属性（V1.1 §9.13）。字段不存在时不写空字符串。
const (
	// 通用凭证与可信等级
	companyAttrCredentialID   = "company.ai.gateway.credential_id"
	companyAttrRequestID      = "company.ai.gateway.request_id"
	companyAttrIdentityMode   = "company.ai.identity.mode"
	companyAttrTargetType     = "company.ai.identity.target_type"
	companyAttrAssurance      = "company.ai.identity.assurance"
	companyAttrIdentityVerify = "company.ai.identity.verified"
	companyAttrCredVerify     = "company.ai.credential.verified"
	companyAttrClientVerify   = "company.ai.client.verified"
	companyAttrEnvironment    = "company.ai.environment"

	// 弱身份个人凭证
	companyAttrPrincipalID       = "company.ai.principal.id"
	companyAttrPrincipalCode     = "company.ai.principal.code"
	companyAttrPrincipalName     = "company.ai.principal.name"
	companyAttrCredentialPurposeID = "company.ai.credential.purpose.id"
	companyAttrCredentialPurpose   = "company.ai.credential.purpose.code"
	companyAttrCredentialPurposeName = "company.ai.credential.purpose.name"
	companyAttrUsageBusinessDomainID = "company.ai.usage_business_domain.id"
	companyAttrUsageBusinessDomain   = "company.ai.usage_business_domain.code"
	companyAttrUsageBusinessDomainName = "company.ai.usage_business_domain.name"
	companyAttrUsageTeamID         = "company.ai.usage_team.id"
	companyAttrUsageTeam           = "company.ai.usage_team.code"
	companyAttrUsageTeamName       = "company.ai.usage_team.name"

	// 强身份 Caller 与应用
	companyAttrCallerID     = "company.ai.caller.id"
	companyAttrRootAppID    = "company.ai.root_app.id"
	companyAttrRootAppName  = "company.ai.root_app.name"
	companyAttrAppBusinessDomainID   = "company.ai.application_business_domain.id"
	companyAttrAppBusinessDomain     = "company.ai.application_business_domain.code"
	companyAttrAppBusinessDomainName = "company.ai.application_business_domain.name"
	companyAttrOwnerTeamID   = "company.ai.owner_team.id"
	companyAttrOwnerTeam     = "company.ai.owner_team.code"
	companyAttrOwnerTeamName = "company.ai.owner_team.name"
	companyAttrRootRunID     = "company.ai.root_run.id"
	companyAttrCurrentExecID = "company.ai.current_execution.id"
	companyAttrParentExecID  = "company.ai.parent_execution.id"
	companyAttrExecType      = "company.ai.execution.type"
	companyAttrExecDepth     = "company.ai.execution.depth"
	companyAttrWorkflowID    = "company.ai.workflow.id"
	companyAttrAgentID       = "company.ai.agent.id"
	companyAttrTaskID        = "company.ai.task.id"
	companyAttrNodeID        = "company.ai.node.id"

	// Identity 自身 Metric（§9.19）
	metricIdentityVerification = "company.ai.identity.verification"
)

var (
	identityVerificationCounter metric.Int64Counter
	identityMetricOnce          sync.Once
)

func initIdentityMetrics() {
	if !IsEnabled() {
		return
	}
	identityMetricOnce.Do(func() {
		meter := otel.Meter("github.com/QuantumNous/new-api/relay/tracing")
		counter, err := meter.Int64Counter(
			metricIdentityVerification,
			metric.WithDescription("企业身份归因验证结果计数"),
		)
		if err == nil {
			identityVerificationCounter = counter
		}
	})
}

// EnterpriseAttributes 从 TrustedAttributionContext 生成企业 Span 属性（§9.13）。
// 返回空切片（而非空字符串值）以避免写入无意义属性。
func EnterpriseAttributes(a *constant.TrustedAttributionContext) []attribute.KeyValue {
	if a == nil {
		return nil
	}
	var attrs []attribute.KeyValue
	add := func(key, val string) {
		if val != "" {
			attrs = append(attrs, attribute.String(key, val))
		}
	}
	addInt := func(key string, v int) {
		if v != 0 {
			attrs = append(attrs, attribute.Int(key, v))
		}
	}
	addBool := func(key string, v bool) {
		if v {
			attrs = append(attrs, attribute.Bool(key, v))
		} else {
			attrs = append(attrs, attribute.Bool(key, false))
		}
	}

	addInt(companyAttrCredentialID, a.TokenID)
	add(companyAttrIdentityMode, a.IdentityMode)
	add(companyAttrTargetType, a.AttributionTarget)
	add(companyAttrAssurance, a.IdentityAssurance)
	addBool(companyAttrIdentityVerify, a.IdentityVerified)
	addBool(companyAttrCredVerify, a.CredentialVerified)
	addBool(companyAttrClientVerify, a.ClientVerified)
	add(companyAttrEnvironment, a.Environment)

	addInt(companyAttrPrincipalID, a.PrincipalID)
	add(companyAttrPrincipalCode, a.PrincipalCode)
	add(companyAttrPrincipalName, a.PrincipalName)
	addInt(companyAttrCredentialPurposeID, a.CredentialPurposeID)
	add(companyAttrCredentialPurpose, a.CredentialPurposeCode)
	add(companyAttrCredentialPurposeName, a.CredentialPurposeName)
	addInt(companyAttrUsageBusinessDomainID, a.UsageBusinessDomainID)
	add(companyAttrUsageBusinessDomain, a.UsageBusinessDomainCode)
	add(companyAttrUsageBusinessDomainName, a.UsageBusinessDomainName)
	addInt(companyAttrUsageTeamID, a.UsageTeamID)
	add(companyAttrUsageTeam, a.UsageTeamCode)
	add(companyAttrUsageTeamName, a.UsageTeamName)

	add(companyAttrCallerID, a.CallerID)
	add(companyAttrRootAppID, a.RootAppID)
	add(companyAttrRootAppName, a.RootAppName)
	addInt(companyAttrAppBusinessDomainID, a.ApplicationBusinessDomainID)
	add(companyAttrAppBusinessDomain, a.ApplicationBusinessDomainCode)
	add(companyAttrAppBusinessDomainName, a.ApplicationBusinessDomainName)
	addInt(companyAttrOwnerTeamID, a.OwnerTeamID)
	add(companyAttrOwnerTeam, a.OwnerTeamCode)
	add(companyAttrOwnerTeamName, a.OwnerTeamName)

	add(companyAttrRootRunID, a.RootRunID)
	add(companyAttrCurrentExecID, a.CurrentExecutionID)
	add(companyAttrParentExecID, a.ParentExecutionID)
	add(companyAttrExecType, a.ExecutionType)
	addInt(companyAttrExecDepth, a.ExecutionDepth)
	add(companyAttrWorkflowID, a.WorkflowID)
	add(companyAttrAgentID, a.AgentID)
	add(companyAttrTaskID, a.TaskID)
	add(companyAttrNodeID, a.NodeID)

	return attrs
}

// recordIdentityMetricsIfAny 记录 company.ai.identity.verification 计数器（§9.19）。
// 高基数维度（principal_id/name、root_run、nonce、request_id、trace_id）不得作为标签。
func recordIdentityMetricsIfAny(a *constant.TrustedAttributionContext, err error) {
	if !IsEnabled() || a == nil {
		return
	}
	initIdentityMetrics()
	if identityVerificationCounter == nil {
		return
	}
	ctx := context.Background()
	labels := []attribute.KeyValue{
		attribute.String("identity_mode", a.IdentityMode),
	}
	if a.IdentityAssurance != "" {
		labels = append(labels, attribute.String(metricAssurance, a.IdentityAssurance))
	}
	if a.CallerID != "" && a.AttributionTarget == constant.AttributionTargetPlatform && a.IdentityVerified {
		labels = append(labels, attribute.String(companyAttrCallerID, a.CallerID))
	}
	if a.CredentialPurposeCode != "" {
		labels = append(labels, attribute.String(companyAttrCredentialPurpose, a.CredentialPurposeCode))
	}
	if a.UsageBusinessDomainCode != "" {
		labels = append(labels, attribute.String(companyAttrUsageBusinessDomain, a.UsageBusinessDomainCode))
	}
	if a.UsageTeamCode != "" {
		labels = append(labels, attribute.String(companyAttrUsageTeam, a.UsageTeamCode))
	}
	if a.FailureReason != "" {
		labels = append(labels, attribute.String("reason_code", a.FailureReason))
	}

	result := "verified"
	if !a.IdentityVerified {
		if a.FailureReason != "" {
			result = "rejected"
		} else {
			result = "unverified"
		}
	}
	labels = append(labels, attribute.String("result", result))
	identityVerificationCounter.Add(ctx, 1, metric.WithAttributes(labels...))
}
