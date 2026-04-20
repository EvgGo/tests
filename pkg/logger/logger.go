package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func SetupLogger(env, postfix, logLevel, logFilePath string) (*slog.Logger, error) {
	var handler slog.Handler

	if logLevel == "localTerminal" {
		handler = slog.NewJSONHandler(os.Stdout, getHandlerOptions(slog.LevelDebug))
	} else {

		dir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("не удалось создать директорию для логов: %w", err)
		}

		logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		handler = slog.NewJSONHandler(logFile, getHandlerOptions(getLogLevel(logLevel)))
	}

	if env == "" {
		slog.New(handler).Error("Неизвестная среда, установлено значение по умолчанию")
	}

	baseLogger := slog.New(handler)
	appCode := fmt.Sprintf("%s%s", env, postfix)
	loggerWithAttrs := baseLogger.With(
		slog.String("env", env),
		slog.String("appCode", appCode),
	)
	return loggerWithAttrs, nil
}

func getLogLevel(logLevel string) slog.Level {
	switch logLevel {
	case "local":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	default:
		return slog.LevelError
	}
}
func getHandlerOptions(level slog.Level) *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				location, _ := time.LoadLocation("Etc/GMT-3")
				return slog.Attr{
					Key:   slog.TimeKey,
					Value: slog.StringValue(time.Now().In(location).Format(time.RFC3339)),
				}
			}
			return a
		},
	}
}
