package logger

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func init() {
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// 情報ログ
func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

// エラーログ
func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

// デバッグログ
func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}
