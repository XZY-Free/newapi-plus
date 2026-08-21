package tracing

import (
	"context"
	"net/http"

	"github.com/QuantumNous/new-api/constant"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// StartBackgroundTaskSpan 为异步任务后台阶段新建 Span，用 Span Link 指向提交阶段的
// W3C Trace（§10.5）。traceparent/tracestate 来自提交阶段持久化的 TraceContextSnapshot。
// 重新附加持久化的 company.ai.* 企业属性（§10.5）。OTel 未启用时返回 no-op span 与同 ctx。
func StartBackgroundTaskSpan(ctx context.Context, name, traceparent, tracestate string, attribution *constant.TrustedAttributionContext) (trace.Span, context.Context) {
	if !IsEnabled() {
		return trace.SpanFromContext(ctx), ctx
	}
	opts := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindInternal)}

	// 用提交阶段持久化的 Trace Context 建立 Span Link。
	if traceparent != "" {
		hdr := http.Header{}
		hdr.Set("traceparent", traceparent)
		if tracestate != "" {
			hdr.Set("tracestate", tracestate)
		}
		submitCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(hdr))
		submitSC := trace.SpanContextFromContext(submitCtx)
		if submitSC.IsValid() {
			opts = append(opts, trace.WithLinks(trace.Link{SpanContext: submitSC}))
		}
	}

	if attrs := EnterpriseAttributes(attribution); len(attrs) > 0 {
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	spanCtx, span := tracer().Start(ctx, name, opts...)
	return span, spanCtx
}
