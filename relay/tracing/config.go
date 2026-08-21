package tracing

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Config 承载第一阶段 OTel 的受控配置（V1.1 §9.3）。
// 只读取 OTel 标准环境变量；不引入业务自定义 OTel 变量，避免配置面膨胀。
type Config struct {
	Enabled bool

	ServiceName string

	ExporterEndpoint string
	// ExporterProtocol 为 "grpc" 或 "http/protobuf"。
	ExporterProtocol string

	// Sampler 语义与 OTEL_TRACES_SAMPLER 保持一致：
	// always_on / always_off / traceidratio / parentbased_traceidratio / 其他（按 traceidratio 兜底）。
	Sampler        string
	SamplerRatio   float64
	hasSamplerArg  bool
}

// DefaultConfig 返回第一阶段默认值：默认关闭 OTel。
func DefaultConfig() Config {
	return Config{
		Enabled:          false,
		ServiceName:      "new-api",
		ExporterProtocol: "grpc",
		Sampler:          "parentbased_traceidratio",
		SamplerRatio:     1.0,
	}
}

// LoadConfig 从环境变量加载 OTel 配置。非法配置（如缺失 endpoint 却要求导出）会导致
// 明确错误，由调用方决定是否允许启动失败（§9.3：配置本身非法导致初始化不能完成时允许启动失败）。
func LoadConfig() (Config, error) {
	cfg := DefaultConfig()

	if common.GetEnvOrDefaultBool("OTEL_ENABLED", false) {
		cfg.Enabled = true
	}

	if v := common.GetEnvOrDefaultString("OTEL_SERVICE_NAME", ""); v != "" {
		cfg.ServiceName = v
	}

	cfg.ExporterEndpoint = common.GetEnvOrDefaultString("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	switch p := strings.ToLower(strings.TrimSpace(common.GetEnvOrDefaultString("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc"))); p {
	case "", "grpc":
		cfg.ExporterProtocol = "grpc"
	case "http/protobuf", "http", "http/proto":
		cfg.ExporterProtocol = "http/protobuf"
	default:
		return cfg, &InvalidConfigError{Message: "unsupported OTEL_EXPORTER_OTLP_PROTOCOL: " + p}
	}

	cfg.Sampler = strings.TrimSpace(common.GetEnvOrDefaultString("OTEL_TRACES_SAMPLER", "parentbased_traceidratio"))
	switch cfg.Sampler {
	case "always_on":
		cfg.SamplerRatio = 1.0
	case "always_off":
		cfg.SamplerRatio = 0.0
	case "traceidratio", "parentbased_traceidratio":
		ratio, ok := parseSamplerArg()
		if ok {
			cfg.SamplerRatio = ratio
			cfg.hasSamplerArg = true
		} else if cfg.SamplerRatio == 0 {
			// 无参数时兜底使用 1.0（第一阶段 POC 建议采样率 1.0）。
			cfg.SamplerRatio = 1.0
		}
	default:
		// 其他字符串：兼容协议要求按 traceidratio 兜底解析参数；无参数则回退 1.0。
		cfg.Sampler = "traceidratio"
		if ratio, ok := parseSamplerArg(); ok {
			cfg.SamplerRatio = ratio
		}
	}

	return cfg, nil
}

func parseSamplerArg() (float64, bool) {
	raw := strings.TrimSpace(common.GetEnvOrDefaultString("OTEL_TRACES_SAMPLER_ARG", ""))
	if raw == "" {
		return 0, false
	}
	ratio, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return ratio, true
}

// InvalidConfigError 表示 OTel 配置非法。
type InvalidConfigError struct {
	Message string
}

func (e *InvalidConfigError) Error() string { return "invalid OpenTelemetry config: " + e.Message }
