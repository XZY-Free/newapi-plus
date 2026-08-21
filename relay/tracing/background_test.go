package tracing

import (
	"context"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// §10.8-10：后台 Span 通过 Span Link 指向提交阶段 Trace，并重新附加企业归因属性。
func TestStartBackgroundTaskSpanLink(t *testing.T) {
	collector := enableTestOTel(t)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))

	// 提交阶段 Trace：产生 traceparent/tracestate。
	submitCtx, submitSpan := otel.Tracer("submit").Start(context.Background(), "async.task.submit",
		trace.WithSpanKind(trace.SpanKindServer))
	submitHdr := http.Header{}
	InjectTraceContext(submitCtx, submitHdr)
	submitSC := submitSpan.SpanContext()

	attr := &constant.TrustedAttributionContext{
		TokenID:               101,
		IdentityMode:          constant.IdentityModeStatic,
		AttributionTarget:     constant.AttributionTargetPrincipal,
		PrincipalID:           1,
		CredentialPurposeID:   10,
		CredentialPurposeCode: "WORKBUDDY",
	}

	bgSpan, spanCtx := StartBackgroundTaskSpan(context.Background(), "async.task.poll",
		submitHdr.Get("traceparent"), submitHdr.Get("tracestate"), attr)

	if !bgSpan.IsRecording() {
		t.Fatal("§10.8-10 后台 Span 应处于 recording 状态")
	}
	// 返回的 spanCtx 应携带该后台 Span 的 SpanContext。
	if got := trace.SpanContextFromContext(spanCtx); !got.IsValid() || got.SpanID() != bgSpan.SpanContext().SpanID() {
		t.Fatalf("§10.8-10 返回 ctx 未携带后台 Span: got %v", got)
	}
	// link-not-join：后台 Span 自身是新 Trace，且非提交 Span 的直接子节点。
	if bgSpan.SpanContext().TraceID() == submitSC.TraceID() {
		t.Fatal("§10.8-10 后台 Span 应为新 Trace（非提交 Trace 的子节点）")
	}

	bgSpan.End()
	spans := collector.snapshot()
	var bgAttrs map[string]attribute.Value
	var bgLinks []sdktrace.Link
	for _, s := range spans {
		if s.Name() == "async.task.poll" {
			bgAttrs = attrMap(s)
			bgLinks = s.Links()
			break
		}
	}
	if bgAttrs == nil {
		t.Fatal("§10.8-10 未导出 async.task.poll Span")
	}
	if len(bgLinks) != 1 {
		t.Fatalf("§10.8-10 应恰好 1 个 Span Link, got %d", len(bgLinks))
	}
	if bgLinks[0].SpanContext.TraceID() != submitSC.TraceID() {
		t.Fatalf("§10.8-10 Link 未指向提交阶段 Trace: got %s want %s",
			bgLinks[0].SpanContext.TraceID(), submitSC.TraceID())
	}
	// 企业归因属性应被重新附加。
	if v, ok := bgAttrs["company.ai.principal.id"]; !ok || v.AsInt64() != 1 {
		t.Fatalf("§10.8-10 企业属性 principal.id 未附加: %v", bgAttrs)
	}
	if v, ok := bgAttrs["company.ai.credential.purpose.code"]; !ok || v.AsString() != "WORKBUDDY" {
		t.Fatalf("§10.8-10 企业属性 purpose.code 未附加: %v", bgAttrs)
	}
}

// OTel 未启用时 StartBackgroundTaskSpan 返回 no-op span 与同 ctx。
func TestStartBackgroundTaskSpanDisabled(t *testing.T) {
	setEnabled(false)
	defer setEnabled(true)

	ctx := context.Background()
	span, spanCtx := StartBackgroundTaskSpan(ctx, "async.task.poll", "00-11111111111111111111111111111111-2222222222222222-01", "", nil)
	if span.IsRecording() {
		t.Fatal("未启用 OTel 时不应 recording")
	}
	if spanCtx != ctx {
		t.Fatal("未启用 OTel 时应返回同 ctx")
	}
}
