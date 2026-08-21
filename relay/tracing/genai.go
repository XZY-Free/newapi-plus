package tracing

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GenAI span 与指标属性键（V1.1 §9.9 / §9.12 / §9.13）。
// 使用字符串常量而非依赖具体 semconv 版本，避免不同 OTel 版本间 GenAI 属性名漂移。
const (
	// 标准 GenAI 属性
	attrGenAIOperationName = "gen_ai.operation.name"
	attrGenAIProviderName  = "gen_ai.provider.name"
	attrGenAIRequestModel  = "gen_ai.request.model"
	attrGenAIResponseModel = "gen_ai.response.model"
	attrGenAIUsageInput    = "gen_ai.usage.input_tokens"
	attrGenAIUsageOutput   = "gen_ai.usage.output_tokens"
	attrGenAIUsageCached   = "gen_ai.usage.input_tokens.cached_tokens"
	attrServerAddress      = "server.address"
	attrErrorType          = "error.type"

	// 企业属性（§9.13 通用）
	attrCompanyQuota    = "company.ai.cost.quota"
	attrCompanyRequest  = "company.ai.gateway.request_id"

	// GenAI 指标
	metricTokenUsage  = "gen_ai.client.token.usage"
	metricDuration    = "gen_ai.client.operation.duration"
	metricTimeToFirst = "gen_ai.client.operation.time_to_first_chunk"

	// 指标标签维度
	metricTokenType      = "token_type"
	metricStatus         = "status"
	metricIdentityMode   = "identity_mode"
	metricAssurance      = "identity_assurance"
)

// GenAISpan 表达一次逻辑模型操作（V1.1 §9.7），覆盖 NewAPI 的 Channel Retry 生命周期。
// OTel 未启用时保持零值语义，所有方法为空操作。
type GenAISpan struct {
	span trace.Span
	ctx  context.Context

	operationName string
	provider      string
	requestModel  string
	requestID     string

	facts *BillingFacts

	start time.Time
}

// StartGenAISpan 建立 GenAI logical CLIENT Span，返回其子 context 供出站请求继承
// （使 Provider HTTP Span 成为其子级，并注入正确 traceparent）。
// OTel 未启用时返回 nil, ctx。
func StartGenAISpan(ctx context.Context, operationName, provider, model, requestID string) (*GenAISpan, context.Context) {
	if !IsEnabled() {
		return nil, ctx
	}
	if operationName == "" {
		operationName = "chat"
	}
	spanCtx, span := tracer().Start(ctx, spanName(operationName, model),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String(attrGenAIOperationName, operationName)),
	)
	if provider != "" {
		span.SetAttributes(attribute.String(attrGenAIProviderName, provider))
	}
	if model != "" {
		span.SetAttributes(attribute.String(attrGenAIRequestModel, model))
	}
	if requestID != "" {
		span.SetAttributes(attribute.String(attrCompanyRequest, requestID))
	}
	// 挂载 BillingFacts，供 Handler 在结算汇聚点写入最终 Usage/Quota（§9.7）。
	spanCtx, facts := newBillingFacts(spanCtx)
	return &GenAISpan{
		span:          span,
		ctx:           spanCtx,
		operationName: operationName,
		provider:      provider,
		requestModel:  model,
		requestID:     requestID,
		facts:         facts,
		start:         time.Now(),
	}, spanCtx
}

func spanName(operationName, model string) string {
	if model == "" {
		return operationName
	}
	return operationName + " " + model
}

// Context 返回 GenAI span 子 context。
func (s *GenAISpan) Context() context.Context {
	if s == nil {
		return context.Background()
	}
	return s.ctx
}

// SetServerAddress 记录出站目标 server.address（Provider base host）。
func (s *GenAISpan) SetServerAddress(address string) {
	if s == nil || s.span == nil || address == "" {
		return
	}
	s.span.SetAttributes(attribute.String(attrServerAddress, address))
}

// End 在模型响应/Usage/最终结算事实可用后结束 Span（§9.7），记录最终 Token、Quota、
// 错误与状态。usage 使用 NewAPI 已归一化 BillingUsage（§9.11）。
// ttfc 为流式 FirstResponseTime 相对请求开始的延迟；只有可靠时传 hasTTFC=true。
func (s *GenAISpan) End(usage *dto.BillingUsage, quota int64, ttfc time.Duration, hasTTFC bool, err error, attribution *constant.TrustedAttributionContext) {
	if s == nil || s.span == nil {
		return
	}
	defer s.span.End()

	// Handler 在结算汇聚点通过 RecordBillingFacts 写入的最终事实优先（§9.7），
	// 调用方参数作为未记录时的回退。
	if s.facts != nil {
		if s.facts.Usage != nil {
			usage = s.facts.Usage
		}
		if s.facts.Quota != 0 {
			quota = s.facts.Quota
		}
		if s.facts.HasTTFC {
			ttfc, hasTTFC = s.facts.TTFC, true
		}
	}

	if attrs := EnterpriseAttributes(attribution); len(attrs) > 0 {
		s.span.SetAttributes(attrs...)
	}
	s.recordTokenAttributes(usage)
	if quota != 0 {
		s.span.SetAttributes(attribute.Int64(attrCompanyQuota, quota))
	}
	if err != nil {
		s.span.SetAttributes(attribute.String(attrErrorType, errorTypeName(err)))
		s.span.SetStatus(codes.Error, err.Error())
	} else {
		s.span.SetStatus(codes.Ok, "")
	}

	recordMetrics(s.start, usage, quota, ttfc, hasTTFC, err, attribution)
	recordIdentityMetricsIfAny(attribution, err)
}

func (s *GenAISpan) recordTokenAttributes(usage *dto.BillingUsage) {
	if usage == nil {
		return
	}
	input, output, cacheRead := NormalizedTokenCounts(usage)
	if input > 0 {
		s.span.SetAttributes(attribute.Int64(attrGenAIUsageInput, input))
	}
	if output > 0 {
		s.span.SetAttributes(attribute.Int64(attrGenAIUsageOutput, output))
	}
	if cacheRead > 0 {
		s.span.SetAttributes(attribute.Int64(attrGenAIUsageCached, cacheRead))
	}
}

// NormalizedTokenCounts 从 NewAPI 归一化 BillingUsage 提取 input/output/cacheRead token 总量
//（V1.1 §9.11）。按 Semantic 选择语义；cached token 是 input 的子集，不再次叠加到 input。
func NormalizedTokenCounts(usage *dto.BillingUsage) (input, output, cacheRead int64) {
	if usage == nil {
		return 0, 0, 0
	}
	switch usage.Semantic {
	case dto.BillingUsageSemanticAnthropic:
		u := usage.ClaudeUsage
		if u == nil {
			return 0, 0, 0
		}
		// 完整有效 input = InputTokens + CacheCreationInputTokens + CacheReadInputTokens。
		input = int64(u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens)
		output = int64(u.OutputTokens)
		cacheRead = int64(u.CacheReadInputTokens)
		return input, output, cacheRead
	case dto.BillingUsageSemanticGemini:
		u := usage.GeminiUsageMetadata
		if u == nil {
			return 0, 0, 0
		}
		// promptTokenCount 已代表有效 Prompt 总量并包含 cached content。
		input = int64(u.PromptTokenCount)
		output = int64(u.CandidatesTokenCount)
		cacheRead = int64(u.CachedContentTokenCount)
		return input, output, cacheRead
	default: // openai / oai responses / 未知
		u := usage.OpenAIUsage
		if u == nil {
			return 0, 0, 0
		}
		// 优先使用归一化 InputTokens/OutputTokens；否则回退 Prompt/Completion。
		if u.InputTokens != 0 {
			input = int64(u.InputTokens)
		} else {
			input = int64(u.PromptTokens)
		}
		if u.OutputTokens != 0 {
			output = int64(u.OutputTokens)
		} else {
			output = int64(u.CompletionTokens)
		}
		if u.PromptTokensDetails.CachedTokens != 0 {
			cacheRead = int64(u.PromptTokensDetails.CachedTokens)
		}
		return input, output, cacheRead
	}
}

func errorTypeName(err error) string {
	if err == nil {
		return ""
	}
	const maxLen = 256
	msg := err.Error()
	if len(msg) > maxLen {
		msg = msg[:maxLen]
	}
	return msg
}
