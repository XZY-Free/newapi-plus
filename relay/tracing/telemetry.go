package tracing

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	initOnce sync.Once
	shutdown func(context.Context) error

	// enabled 记录 OTel 是否在本进程启用。OTEL_ENABLED=false 时，
	// 所有辅助函数都应短路，不改变模型请求成功/失败语义（§9.3）。
	enabled   bool
	enabledMu sync.RWMutex
)

// IsEnabled 返回 OTel 是否已启用。
func IsEnabled() bool {
	enabledMu.RLock()
	defer enabledMu.RUnlock()
	return enabled
}

func setEnabled(v bool) {
	enabledMu.Lock()
	enabled = v
	enabledMu.Unlock()
}

// Init 在 InitResources 中、HTTP Server 接收请求前调用（§9.4）。
// 它在 Logger 初始化之后执行。OTEL_ENABLED=false 时为空操作，不会改变模型调用语义。
// 配置非法导致无法完成初始化时返回错误，由调用方决定是否允许启动失败。
func Init(ctx context.Context) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		setEnabled(false)
		common.SysLog("OpenTelemetry is disabled (OTEL_ENABLED=false)")
		return nil
	}

	var initErr error
	initOnce.Do(func() {
		initErr = initEnabled(ctx, cfg)
	})
	if initErr != nil {
		setEnabled(false)
		return initErr
	}

	setEnabled(true)

	// 全局 Propagator 使用 W3C Trace Context（§9.5）。
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
	))
	common.SysLog(fmt.Sprintf("OpenTelemetry enabled: service=%s endpoint=%s protocol=%s sampler=%s",
		cfg.ServiceName, cfg.ExporterEndpoint, cfg.ExporterProtocol, cfg.Sampler))
	return nil
}

func initEnabled(ctx context.Context, cfg Config) error {
	if cfg.ExporterEndpoint == "" {
		return &InvalidConfigError{Message: "OTEL_ENABLED=true 但 OTEL_EXPORTER_OTLP_ENDPOINT 为空"}
	}

	exp, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return err
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(attribute.String("service.name", cfg.ServiceName)),
	)
	if err != nil {
		return fmt.Errorf("create otel resource failed: %w", err)
	}

	sp, err := newTraceProvider(ctx, cfg, exp, res)
	if err != nil {
		return err
	}
	otel.SetTracerProvider(sp)

	mp, err := newMeterProvider(ctx, cfg, res)
	if err != nil {
		return err
	}
	otel.SetMeterProvider(mp)

	shutdown = func(ctx context.Context) error {
		var firstErr error
		if err := mp.Shutdown(ctx); err != nil {
			firstErr = err
		}
		if err := sp.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		return firstErr
	}
	return nil
}

func newMeterProvider(ctx context.Context, cfg Config, res *sdkresource.Resource) (*sdkmetric.MeterProvider, error) {
	var exp sdkmetric.Exporter
	var err error
	if cfg.ExporterProtocol == "http/protobuf" {
		exp, err = otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpointURL(cfg.ExporterEndpoint),
			otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
		)
	} else {
		exp, err = otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpointURL(cfg.ExporterEndpoint),
			otlpmetricgrpc.WithCompressor("gzip"),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("create otel metric exporter failed: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(60*time.Second))),
	)
	return mp, nil
}

func newTraceExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	if cfg.ExporterProtocol == "http/protobuf" {
		return otlptracehttp.New(ctx,
			otlptracehttp.WithEndpointURL(cfg.ExporterEndpoint),
			otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
		)
	}
	return otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpointURL(cfg.ExporterEndpoint),
		otlptracegrpc.WithCompressor("gzip"),
	)
}

func newTraceProvider(ctx context.Context, cfg Config, exp sdktrace.SpanExporter, res *sdkresource.Resource) (*sdktrace.TracerProvider, error) {
	sp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
	)
	return sp, nil
}

// InjectTraceContext 使用全局 Propagator 将 W3C Trace Context 注入目标 Header（§9.17）。
// OTel 未启用时 Propagator 为 no-op，不注入任何 Header，不改变现有行为。
func InjectTraceContext(ctx context.Context, hdr http.Header) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(hdr))
}

// ServerMiddleware 返回 Gin HTTP Server Span 中间件（§9.6）。
// 必须放置在 NewAPI RequestId 中间件之后、路由与 TokenAuth/AIIdentityAuth 之前。
// OTel 未启用时返回透传中间件，不改变任何行为。
func ServerMiddleware() gin.HandlerFunc {
	if !IsEnabled() {
		return func(c *gin.Context) { c.Next() }
	}
	return otelgin.Middleware("new-api")
}

// Shutdown 在主程序 graceful shutdown 时调用（§9.4），受既有 shutdown timeout 控制。
// OTel 未启用时为空操作。
func Shutdown(ctx context.Context) {
	if !IsEnabled() || shutdown == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		if err := shutdown(ctx); err != nil {
			common.SysError("OpenTelemetry shutdown failed: " + err.Error())
		}
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		common.SysError("OpenTelemetry shutdown timed out")
	}
}
