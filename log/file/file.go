// Package file 为 log 提供按大小轮转的文件 writer。
package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const defaultMaxSize = 100

var _ io.WriteCloser = (*lumberjack.Logger)(nil)

// Option 配置 [NewWriter]。
type Option func(*lumberjack.Logger)

// WithMaxSize 设置轮转前的最大大小，单位兆字节。默认为 100。最后一次生效。
func WithMaxSize(megabytes int) Option {
	return func(writer *lumberjack.Logger) { writer.MaxSize = megabytes }
}

// WithMaxBackups 设置保留的旧文件最大个数。默认为 0，表示不按数量删除。最后一次生效。
func WithMaxBackups(count int) Option {
	return func(writer *lumberjack.Logger) { writer.MaxBackups = count }
}

// WithMaxAge 设置旧文件保留的最大天数。默认为 0，表示不按天数删除。最后一次生效。
func WithMaxAge(days int) Option {
	return func(writer *lumberjack.Logger) { writer.MaxAge = days }
}

// WithCompress 打开或关闭轮转文件的 gzip 压缩。最后一次生效；默认关闭。
func WithCompress(enabled bool) Option {
	return func(writer *lumberjack.Logger) { writer.Compress = enabled }
}

// WithLocalTime 用本地时间而不是 UTC 生成备份文件名。默认为 true。最后一次生效。
func WithLocalTime(enabled bool) Option {
	return func(writer *lumberjack.Logger) { writer.LocalTime = enabled }
}

// NewWriter 校验后返回原始 [*lumberjack.Logger]。filename 在扩展名前拼接主机名和 pid。
// lumberjack 延迟打开文件，目录或权限错误可能在首次 Write 才返回。
func NewWriter(filename string, options ...Option) (*lumberjack.Logger, error) {
	unique, err := uniqueFilename(filename)
	if err != nil {
		return nil, err
	}
	writer := &lumberjack.Logger{
		Filename:  unique,
		MaxSize:   defaultMaxSize,
		LocalTime: true,
	}
	for _, option := range options {
		if option != nil {
			option(writer)
		}
	}
	if err := validate(writer); err != nil {
		return nil, err
	}
	return writer, nil
}

func validate(writer *lumberjack.Logger) error {
	if writer.Filename == "" {
		return fmt.Errorf("file: filename must not be empty")
	}
	if writer.MaxSize <= 0 {
		return fmt.Errorf("file: maxSize must be > 0")
	}
	if writer.MaxBackups < 0 {
		return fmt.Errorf("file: maxBackups must be >= 0")
	}
	if writer.MaxAge < 0 {
		return fmt.Errorf("file: maxAge must be >= 0")
	}
	return nil
}

func uniqueFilename(filename string) (string, error) {
	if filename == "" {
		return "", fmt.Errorf("file: filename must not be empty")
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	host = strings.Map(func(r rune) rune {
		if r == filepath.Separator || r == '/' || r == '\\' || r < 32 {
			return '-'
		}
		return r
	}, host)
	if host == "" {
		host = "unknown"
	}
	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)
	return fmt.Sprintf("%s-%s-%d%s", stem, host, os.Getpid(), ext), nil
}
