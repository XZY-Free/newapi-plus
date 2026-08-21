package channel

import "testing"

// §9.20-7：Channel wildcard/regex passthrough 不得原样复制入站 traceparent/tracestate。
// 该守卫通过 passthroughSkipHeaderNamesLower 生效，OTel Propagator 只基于当前 Child Span 注入。
func TestTracingHeadersNotPassthrough(t *testing.T) {
	for _, h := range []string{"traceparent", "tracestate", "Traceparent", "TRACEPARENT", "Tracestate"} {
		if !shouldSkipPassthroughHeader(h) {
			t.Fatalf("§9.20-7 头 %q 应被 passthrough 跳过", h)
		}
	}
	if _, ok := passthroughSkipHeaderNamesLower["traceparent"]; !ok {
		t.Fatalf("§9.20-7 缺少 traceparent 跳过条目")
	}
	if _, ok := passthroughSkipHeaderNamesLower["tracestate"]; !ok {
		t.Fatalf("§9.20-7 缺少 tracestate 跳过条目")
	}
}
