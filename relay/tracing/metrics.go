package tracing

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	genaiTokenUsage metric.Int64Histogram
	genaiDuration   metric.Int64Histogram
	genaiTTFC       metric.Int64Histogram
	genaiMetricsOnce sync.Once
)

func tracer() trace.Tracer {
	return otel.Tracer("github.com/QuantumNous/new-api/relay/tracing")
}

// initGenAIMetrics 惰性创建 GenAI 指标。OTel 未启用时为 no-op。
func initGenAIMetrics() {
	if !IsEnabled() {
		return
	}
	genaiMetricsOnce.Do(func() {
		meter := otel.Meter("github.com/QuantumNous/new-api/relay/tracing")
		var err error
		genaiTokenUsage, err = meter.Int64Histogram(
			metricTokenUsage,
			metric.WithUnit("By"),
			metric.WithDescription("GenAI client token usage by token type"),
		)
		if err != nil {
			return
		}
		genaiDuration, err = meter.Int64Histogram(
			metricDuration,
			metric.WithUnit("ms"),
			metric.WithDescription("GenAI client logical operation duration"),
		)
		if err != nil {
			return
		}
		genaiTTFC, _ = meter.Int64Histogram(
			metricTimeToFirst,
			metric.WithUnit("ms"),
			metric.WithDescription("GenAI client streaming time to first chunk"),
		)
	})
}

// metricLabelsFromAttribution 构建受控低基数指标标签（V1.1 §9.12）。
// 不得包含 principal_id/name、request_id、trace_id、span_id 等高基数维度。
func metricLabelsFromAttribution(a *constant.TrustedAttributionContext) []attribute.KeyValue {
	if a == nil {
		return nil
	}
	var attrs []attribute.KeyValue
	appendIf := func(key string, v string) {
		if v != "" {
			attrs = append(attrs, attribute.String(key, v))
		}
	}
	// caller 仅强身份且低基数平台 Caller（弱身份/个人凭证不作为常规标签）。
	if a.CallerID != "" && a.AttributionTarget == constant.AttributionTargetPlatform {
		appendIf(companyAttrCallerID, a.CallerID)
	}
	appendIf(companyAttrRootAppID, a.RootAppID)
	appendIf(companyAttrAppBusinessDomainID, intID(a.ApplicationBusinessDomainID))
	appendIf(companyAttrOwnerTeamID, intID(a.OwnerTeamID))
	appendIf(companyAttrUsageBusinessDomainID, intID(a.UsageBusinessDomainID))
	appendIf(companyAttrUsageTeamID, intID(a.UsageTeamID))
	appendIf(companyAttrCredentialPurposeID, intID(a.CredentialPurposeID))
	appendIf(companyAttrAssurance, a.IdentityAssurance)
	return attrs
}

func intID(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(int64(v), 10)
}

// recordMetrics 记录 GenAI 指标。仅当 OTel 启用时工作。
func recordMetrics(start time.Time, usage *dto.BillingUsage, quota int64, ttfc time.Duration, hasTTFC bool, err error, attribution *constant.TrustedAttributionContext) {
	if !IsEnabled() {
		return
	}
	initGenAIMetrics()
	if genaiDuration == nil {
		return
	}
	ctx := context.Background()
	labels := metricLabelsFromAttribution(attribution)
	status := "ok"
	if err != nil {
		status = "error"
	}

	genaiDuration.Record(ctx, time.Since(start).Milliseconds(),
		metric.WithAttributes(labels...), metric.WithAttributes(attribute.String(metricStatus, status)))

	if hasTTFC && genaiTTFC != nil {
		genaiTTFC.Record(ctx, ttfc.Milliseconds(), metric.WithAttributes(labels...))
	}

	if usage != nil && genaiTokenUsage != nil {
		input, output, cacheRead := NormalizedTokenCounts(usage)
		if input > 0 {
			genaiTokenUsage.Record(ctx, input, metric.WithAttributes(labels...), metric.WithAttributes(attribute.String(metricTokenType, "input")))
		}
		if output > 0 {
			genaiTokenUsage.Record(ctx, output, metric.WithAttributes(labels...), metric.WithAttributes(attribute.String(metricTokenType, "output")))
		}
		if cacheRead > 0 {
			genaiTokenUsage.Record(ctx, cacheRead, metric.WithAttributes(labels...), metric.WithAttributes(attribute.String(metricTokenType, "cache_read")))
		}
	}
}
