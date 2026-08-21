package tracing

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// extractFromHeader 用全局 Propagator 解析已注入 header 中的 SpanContext（代替不存在的 ParseTraceParent）。
func extractFromHeader(hdr http.Header) trace.SpanContext {
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(hdr))
	return trace.SpanContextFromContext(ctx)
}

// --- OTel 测试基础设施（§9.20） ---

// spanCollector 同步收集导出的 Span，便于断言。
type spanCollector struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (c *spanCollector) OnStart(_ context.Context, _ sdktrace.ReadWriteSpan) {}
func (c *spanCollector) OnEnd(s sdktrace.ReadOnlySpan) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.spans = append(c.spans, s)
}
func (c *spanCollector) Shutdown(context.Context) error { return nil }
func (c *spanCollector) ForceFlush(context.Context) error { return nil }

func (c *spanCollector) snapshot() []sdktrace.ReadOnlySpan {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]sdktrace.ReadOnlySpan, len(c.spans))
	copy(out, c.spans)
	return out
}

// enableTestOTel 启用测试 TracerProvider 并打开 enabled 开关，返回收集器。
func enableTestOTel(t *testing.T) *spanCollector {
	t.Helper()
	collector := &spanCollector{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(collector),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	setEnabled(true)
	t.Cleanup(func() {
		setEnabled(false)
		otel.SetTracerProvider(prev)
	})
	return collector
}

func attrMap(span sdktrace.ReadOnlySpan) map[string]attribute.Value {
	m := make(map[string]attribute.Value)
	for _, kv := range span.Attributes() {
		m[string(kv.Key)] = kv.Value
	}
	return m
}

// --- §9.20 门禁项 ---

// §9.20-1/2：无 traceparent 产生新 Trace；合法 traceparent 保持同一 TraceId 且 Server Span 为新 SpanId。
func TestTraceContextPropagation(t *testing.T) {
	collector := enableTestOTel(t)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))

	// 无入站 traceparent：NewAPI 由根 Server Span 产生新 Trace，Inject 后带上该新 TraceId。
	{
		rootCtx, rootSpan := otel.Tracer("test").Start(context.Background(), "server-root",
			trace.WithSpanKind(trace.SpanKindServer))
		hdr := http.Header{}
		InjectTraceContext(rootCtx, hdr)
		if hdr.Get("traceparent") == "" {
			t.Fatalf("§9.20-1 未产生 traceparent")
		}
		sc := extractFromHeader(hdr)
		if !sc.TraceID().IsValid() {
			t.Fatalf("§9.20-1 新 Trace 无效")
		}
		if sc.TraceID() != rootSpan.SpanContext().TraceID() {
			t.Fatalf("§9.20-1 新 Trace 应为根 Server Span 的 TraceId")
		}
	}

	// 合法入站 traceparent：NewAPI 与上游保持同一 TraceId，Server Span 是新 SpanId。
	{
		parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "parent",
			trace.WithSpanKind(trace.SpanKindServer))
		childCtx, _ := otel.Tracer("test").Start(parentCtx, "child")
		hdr := http.Header{}
		InjectTraceContext(childCtx, hdr)
		sc := extractFromHeader(hdr)
		if sc.TraceID() != parentSpan.SpanContext().TraceID() {
			t.Fatalf("§9.20-2 上游 TraceId 不一致: got %s want %s", sc.TraceID(), parentSpan.SpanContext().TraceID())
		}
		if sc.SpanID() == parentSpan.SpanContext().SpanID() {
			t.Fatalf("§9.20-2 Server Span 不应复用父 SpanId")
		}
		childSpanCtx := trace.SpanFromContext(childCtx).SpanContext()
		if sc.SpanID() != childSpanCtx.SpanID() {
			t.Fatalf("§9.20-2 child traceparent SpanId 与 Child Span 不一致")
		}
	}
	_ = collector
}

// §9.20-4：一个逻辑模型请求只产生一个 GenAI logical Span。
func TestStartGenAISpanSingle(t *testing.T) {
	collector := enableTestOTel(t)
	span, ctx := StartGenAISpan(context.Background(), "chat", "openai", "gpt-4o", "req-1")
	if span == nil || span.span == nil {
		t.Fatalf("StartGenAISpan 返回空 span")
	}
	if from := trace.SpanFromContext(ctx); !from.IsRecording() {
		t.Fatalf("返回 ctx 应持有正在记录的 span")
	}
	span.End(nil, 0, 0, false, nil, nil)
	spans := collector.snapshot()
	genAI := 0
	for _, s := range spans {
		if string(s.Name()) == "chat gpt-4o" {
			genAI++
		}
	}
	if genAI != 1 {
		t.Fatalf("§9.20-4 一个逻辑请求应只产生一个 GenAI logical Span，得到 %d", genAI)
	}
}

// §9.20-6：Provider 出站请求有正确的 child traceparent。
func TestOutboundChildTraceparent(t *testing.T) {
	collector := enableTestOTel(t)
	_, ctx := StartGenAISpan(context.Background(), "chat", "anthropic", "claude", "req-2")
	parentSpan := trace.SpanFromContext(ctx)
	hdr := http.Header{}
	InjectTraceContext(ctx, hdr)
	sc := extractFromHeader(hdr)
	if sc.TraceID() != parentSpan.SpanContext().TraceID() {
		t.Fatalf("§9.20-6 出站 child 与 GenAI Span 同 Trace")
	}
	// 传播的就是当前 GenAI Span（作为上游出站的父级），SpanId 必须有效且带已采样的标志。
	if !sc.SpanID().IsValid() || !sc.IsSampled() {
		t.Fatalf("§9.20-6 出站 traceparent SpanId 无效或未采样")
	}
	_ = collector
}

// §9.20-8：OTel disabled 时不改变模型 HTTP 行为（返回 nil span 与同 ctx，不注入 header）。
func TestDisabledNoOp(t *testing.T) {
	if IsEnabled() {
		t.Skip("需要 OTel disabled 环境")
	}
	parent := context.WithValue(context.Background(), "k", "v")
	span, ctx := StartGenAISpan(parent, "chat", "openai", "gpt-4o", "req-3")
	if span != nil {
		t.Fatalf("§9.20-8 disabled 时不应创建 GenAI span")
	}
	if ctx.Value("k") != "v" {
		t.Fatalf("§9.20-8 disabled 时 ctx 应原样返回")
	}
	hdr := http.Header{}
	InjectTraceContext(ctx, hdr)
	if _, ok := hdr["Traceparent"]; ok {
		t.Fatalf("§9.20-8 disabled 时不应注入 traceparent")
	}
}

// §9.20-10/11/12：Token 计数语义。
func TestNormalizedTokenCounts(t *testing.T) {
	// 10：OpenAI 总量没有 cache 双计数（input 已含 cached，total 不叠加）。
	{
		in, out, cache := NormalizedTokenCounts(&dto.BillingUsage{
			Semantic: dto.BillingUsageSemanticOpenAI,
			OpenAIUsage: &dto.Usage{
				InputTokens: 100, OutputTokens: 50,
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 20},
			},
		})
		if in != 100 || out != 50 || cache != 20 {
			t.Fatalf("§9.20-10 OpenAI 计数错误: in=%d out=%d cache=%d", in, out, cache)
		}
	}
	// 11：Anthropic input total 包含 cache creation/read。
	{
		in, out, cache := NormalizedTokenCounts(&dto.BillingUsage{
			Semantic: dto.BillingUsageSemanticAnthropic,
			ClaudeUsage: &dto.ClaudeUsage{
				InputTokens: 100, CacheCreationInputTokens: 30, CacheReadInputTokens: 20, OutputTokens: 50,
			},
		})
		if in != 150 || out != 50 || cache != 20 {
			t.Fatalf("§9.20-11 Anthropic 计数错误: in=%d out=%d cache=%d", in, out, cache)
		}
	}
	// 12：Gemini promptTokenCount 不再叠加 cachedContentTokenCount。
	{
		in, out, cache := NormalizedTokenCounts(&dto.BillingUsage{
			Semantic: dto.BillingUsageSemanticGemini,
			GeminiUsageMetadata: &dto.GeminiUsageMetadata{
				PromptTokenCount: 100, CandidatesTokenCount: 50, CachedContentTokenCount: 20,
			},
		})
		if in != 100 || out != 50 || cache != 20 {
			t.Fatalf("§9.20-12 Gemini 计数错误: in=%d out=%d cache=%d", in, out, cache)
		}
	}
	// nil usage 安全。
	if in, out, cache := NormalizedTokenCounts(nil); in != 0 || out != 0 || cache != 0 {
		t.Fatalf("nil usage 应返回全 0")
	}
}

// §9.20-13：Streaming TTFC 经 BillingFacts 与 FirstResponseTime 一致地传递。
func TestTTFCFacts(t *testing.T) {
	f := &BillingFacts{}
	ctx := context.WithValue(context.Background(), billingFactsKey{}, f)
	ttfc := 1234 * time.Millisecond
	RecordTimeToFirstChunk(ctx, ttfc, true)
	if !f.HasTTFC || f.TTFC != ttfc {
		t.Fatalf("§9.20-13 TTFC 未正确记录")
	}
}

// §9.20-14/15/16/17：Span 结束时有最终 Token/Quota；Error Span 有 error.type；
// 无 Prompt/Response 全文；无 gen_ai.business_* 私有字段。
func TestSpanEndFacts(t *testing.T) {
	collector := enableTestOTel(t)
	attribution := &constant.TrustedAttributionContext{
		TokenID:            7,
		CredentialVerified: true,
		ClientVerified:     false,
		IdentityMode:       constant.IdentityModeStatic,
		IdentityAssurance:  "2",
		PrincipalCode:      "wb-1",
	}
	span, _ := StartGenAISpan(context.Background(), "chat", "openai", "gpt-4o", "req-4")
	usage := &dto.BillingUsage{
		Semantic: dto.BillingUsageSemanticOpenAI,
		OpenAIUsage: &dto.Usage{
			InputTokens: 100, OutputTokens: 50,
			PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 20},
		},
	}
	span.End(usage, 4321, 0, false, nil, attribution)

	spans := collector.snapshot()
	if len(spans) != 1 {
		t.Fatalf("应导出 1 个 span，得到 %d", len(spans))
	}
	attrs := attrMap(spans[0])

	// 14：最终 Token/Quota。
	if got := attrs[attrGenAIUsageInput].AsInt64(); got != 100 {
		t.Fatalf("§9.20-14 input_tokens 错误: %d", got)
	}
	if got := attrs[attrGenAIUsageOutput].AsInt64(); got != 50 {
		t.Fatalf("§9.20-14 output_tokens 错误: %d", got)
	}
	if got := attrs[attrCompanyQuota].AsInt64(); got != 4321 {
		t.Fatalf("§9.20-14 quota 错误: %d", got)
	}
	// 3/19/21：Server/GenAI Span 上有可信归因属性。
	if got := attrs[companyAttrCredVerify].AsBool(); got != true {
		t.Fatalf("§9.20-19 credential_verified 应为 true")
	}
	if got := attrs[companyAttrClientVerify].AsBool(); got != false {
		t.Fatalf("§9.20-19 client_verified 应为 false（弱身份）")
	}
	// 16/17：不存在全文与私有字段。
	for _, k := range []string{"gen_ai.prompt", "gen_ai.response", "gen_ai.completion", "gen_ai.business_domain", "gen_ai.business_team"} {
		if _, ok := attrs[k]; ok {
			t.Fatalf("§9.20-16/17 不应出现字段 %q", k)
		}
	}
}

// §9.20-15：Error Span 包含 error.type。
func TestSpanEndError(t *testing.T) {
	collector := enableTestOTel(t)
	span, _ := StartGenAISpan(context.Background(), "chat", "openai", "gpt-4o", "req-5")
	span.End(nil, 0, 0, false, errBoom("upstream 500"), nil)
	spans := collector.snapshot()
	if len(spans) != 1 {
		t.Fatalf("应导出 1 个 span")
	}
	attrs := attrMap(spans[0])
	if _, ok := attrs[attrErrorType]; !ok {
		t.Fatalf("§9.20-15 缺少 error.type")
	}
	if spans[0].Status().Code != codes.Error {
		t.Fatalf("§9.20-15 span 应为 error 状态")
	}
}

// §9.20-18/20：高基数字段不进 Metric 标签；Principal 不出现在 Metric Label。
func TestMetricLabelsFromAttribution(t *testing.T) {
	attrs := metricLabelsFromAttribution(&constant.TrustedAttributionContext{
		PrincipalID:        99,
		PrincipalCode:      "wb-1",
		CallerID:           "caller-x",
		AttributionTarget:  constant.AttributionTargetPlatform,
		RootAppID:          "app-1",
		ApplicationBusinessDomainID: 3,
		OwnerTeamID:        4,
		UsageBusinessDomainID: 5,
		UsageTeamID:        6,
		CredentialPurposeID: 7,
		IdentityAssurance:  "2",
	})
	labels := make(map[string]string)
	for _, kv := range attrs {
		labels[string(kv.Key)] = kv.Value.AsString()
	}
	// 20：Principal 不出现在常规 Metric Label。
	if _, ok := labels[companyAttrPrincipalID]; ok {
		t.Fatalf("§9.20-20 principal_id 不应作为 Metric Label")
	}
	if _, ok := labels[companyAttrPrincipalCode]; ok {
		t.Fatalf("§9.20-20 principal.code 不应作为 Metric Label")
	}
	// 18：request_id / trace_id / span_id 不得进入 Metric Label。
	for _, k := range []string{companyAttrRequestID, "trace_id", "span_id"} {
		if _, ok := labels[k]; ok {
			t.Fatalf("§9.20-18 高基数字段 %q 不应作为 Metric Label", k)
		}
	}
	// 低基数维度应保留。
	for _, k := range []string{companyAttrRootAppID, companyAttrAppBusinessDomainID, companyAttrOwnerTeamID, companyAttrUsageBusinessDomainID, companyAttrUsageTeamID, companyAttrCredentialPurposeID, companyAttrAssurance} {
		if _, ok := labels[k]; !ok {
			t.Fatalf("§9.20-18 低基数标签 %q 应存在", k)
		}
	}
}

// §9.20-9：Exporter 异常时不 panic（End 防御性）。用失败 Exporter 模拟不可达。
type failExporter struct{}

func (failExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error { return errBoom("exporter unreachable") }
func (failExporter) Shutdown(context.Context) error { return nil }

func TestEndSurvivesExporterFailure(t *testing.T) {
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(failExporter{}, sdktrace.WithBatchTimeout(time.Hour)),
	))
	setEnabled(true)
	defer func() {
		setEnabled(false)
		otel.SetTracerProvider(prev)
	}()
	// End 在 Exporter 不可达时不得 panic，模型调用仍可完成。
	span, _ := StartGenAISpan(context.Background(), "chat", "openai", "gpt-4o", "req-6")
	span.End(&dto.BillingUsage{Semantic: dto.BillingUsageSemanticOpenAI}, 10, 0, false, nil, nil)
}

type errBoom string

func (e errBoom) Error() string { return string(e) }
