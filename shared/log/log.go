package log

import (
	"log/slog"
	"os"
	"strings"
)

const (
	EnvLogLevel     = "LOG_LEVEL"
	DefaultLogLevel = slog.LevelInfo
)

type Opts struct {
	ServiceName string
	Level       slog.Level
	AddSource   bool
	JSON        bool
}

func Setup(o Opts) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     o.Level,
		AddSource: o.AddSource,
	}

	if o.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	// Set service name to all log entries
	attrs := []slog.Attr{slog.String("service", o.ServiceName)}
	handler = handler.WithAttrs(attrs)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

func SetupFromEnv(serviceName string) *slog.Logger {
	return Setup(Opts{
		ServiceName: serviceName,
		Level:       GetLogLevelFromEnv(),
		AddSource:   GetLogLevelFromEnv() <= slog.LevelDebug, // When debug, add source file/line info
		JSON:        true,
	})
}

func GetLogLevelFromEnv() slog.Level {
	levelStr := os.Getenv(EnvLogLevel)
	if levelStr == "" {
		return DefaultLogLevel
	}

	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return DefaultLogLevel
	}
}
