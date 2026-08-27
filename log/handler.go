// Package log 提供标准 [slog.Handler] 的构建与装饰。应用程序继续通过 [slog.Logger] 打日志。
package log

import (
	"io"
	"log/slog"
	"os"
)

// Format 选择基座编码。零值与未知值回退为 FormatText。
type Format int

const (
	// FormatText 使用 [slog.NewTextHandler] 写出记录。
	FormatText Format = iota
	// FormatJSON 使用 [slog.NewJSONHandler] 写出记录。
	FormatJSON
)

// Option 配置 [NewHandler]。
type Option func(*handlerConfig)

type handlerConfig struct {
	writer            io.Writer
	format            Format
	level             slog.Leveler
	source            bool
	attrGroup         string
	contextExtractors []ContextExtractor
	redactKeys        []string
	dropFunc          DropFunc
	sampling          SamplingConfig
	truncateMaxBytes  int
	errorFunc         ErrorFunc
}

// WithWriter 设置基座 writer。默认为 [os.Stderr]。
func WithWriter(writer io.Writer) Option {
	return func(config *handlerConfig) { config.writer = writer }
}

// WithFormat 选择 text 或 JSON。默认为 [FormatText]。最后一次生效。
func WithFormat(format Format) Option {
	return func(config *handlerConfig) { config.format = format }
}

// WithLevel 设置最低级别。最后一次生效；nil 使用 slog 默认，当前为 [slog.LevelInfo]。
func WithLevel(level slog.Leveler) Option {
	return func(config *handlerConfig) { config.level = level }
}

// WithSource 打开或关闭基座 source。最后一次生效；默认关闭。
func WithSource(enabled bool) Option {
	return func(config *handlerConfig) { config.source = enabled }
}

// WithAttrGroup 将本次调用属性收入指定 group。time/level/msg、预绑定、Context、source、log.truncated、log.suppressed 仍在根上。
// 空 group 禁用。最后一次生效。
func WithAttrGroup(group string) Option {
	return func(config *handlerConfig) { config.attrGroup = group }
}

// WithContextExtractor 追加从 context 抽取 attrs 的函数，接在默认 [ContextWithAttrs] 抽取器之后。
// 多次调用累加；nil 跳过。
func WithContextExtractor(extractors ...ContextExtractor) Option {
	return func(config *handlerConfig) {
		for _, extractor := range extractors {
			if extractor != nil {
				config.contextExtractors = append(config.contextExtractors, extractor)
			}
		}
	}
}

// WithRedactKey 按叶子名或点分组路径脱敏。多次调用累加；空字符串忽略。
func WithRedactKey(keys ...string) Option {
	return func(config *handlerConfig) {
		config.redactKeys = append(config.redactKeys, keys...)
	}
}

// WithDropFunc 设置谓词，返回 true 时丢弃记录。最后一次生效；nil 禁用。
func WithDropFunc(dropFunc DropFunc) Option {
	return func(config *handlerConfig) { config.dropFunc = dropFunc }
}

// WithSampling 为 Warn 和 Error 配置采样。最后一次生效。语义见 [SamplingConfig]。
func WithSampling(sampling SamplingConfig) Option {
	return func(config *handlerConfig) {
		config.sampling = sampling
	}
}

// WithTruncate 限制 message 以及 string / []byte 的字节大小。字符串 UTF-8 安全。
// maxBytes <= 0 禁用。最后一次生效。
func WithTruncate(maxBytes int) Option {
	return func(config *handlerConfig) {
		config.truncateMaxBytes = maxBytes
	}
}

// WithErrorFunc 设置基座 error / panic 回调。未配置或 nil 不回调，仍恢复基座 panic。最后一次生效。
func WithErrorFunc(errorFunc ErrorFunc) Option {
	return func(config *handlerConfig) { config.errorFunc = errorFunc }
}

// NewHandler 构建组合后的 [slog.Handler]。默认 text / Info / stderr，合并 [ContextWithAttrs]，恢复基座 panic。
// 其余装饰按 Option 叠加。WithWriter(nil) 构造时 panic。
func NewHandler(options ...Option) slog.Handler {
	config := &handlerConfig{
		writer:            os.Stderr,
		format:            FormatText,
		level:             slog.LevelInfo,
		contextExtractors: []ContextExtractor{attrsFromContext},
	}
	for _, option := range options {
		if option != nil {
			option(config)
		}
	}
	if config.writer == nil {
		panic("log writer is nil")
	}
	handler := newBaseHandler(config)
	return newComposedHandler(handler, config)
}

func newBaseHandler(config *handlerConfig) slog.Handler {
	options := &slog.HandlerOptions{Level: config.level, AddSource: config.source}
	switch config.format {
	case FormatText:
		return slog.NewTextHandler(config.writer, options)
	case FormatJSON:
		return slog.NewJSONHandler(config.writer, options)
	default:
		return slog.NewTextHandler(config.writer, options)
	}
}

func newComposedHandler(handler slog.Handler, config *handlerConfig) slog.Handler {
	// 由内向外包装，Handle 顺序：AttrGroup → Context → Redact → Drop → Sampling → Truncate → ErrorFunc → base。
	handler = newErrorHandler(handler, config.errorFunc)
	handler = newTruncateHandler(handler, config.truncateMaxBytes)
	handler = newSamplingHandler(handler, config.sampling)
	handler = newDropHandler(handler, config.dropFunc)
	handler = newRedactHandler(handler, config.redactKeys...)
	handler = newContextHandler(handler, config.contextExtractors...)
	handler = newAttrGroupHandler(handler, config.attrGroup)
	return handler
}
