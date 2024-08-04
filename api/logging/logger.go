// api/logging/logger.go

package util

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func InitLogger(logDirPath string) {
	config := zap.NewProductionConfig()

	// Ensure log directory exists
	err := os.MkdirAll(logDirPath, 0755)
	if err != nil {
		panic(err)
	}

	// Customize log level based on environment
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel != "" {
		level, err := zapcore.ParseLevel(logLevel)
		if err == nil {
			config.Level.SetLevel(level)
		}
	}

	logFilePath := filepath.Join(logDirPath, "api.log")
	logErrorFilePath := filepath.Join(logDirPath, "api_error.log")

	// Customize output paths
	config.OutputPaths = []string{"stdout", logFilePath}
	config.ErrorOutputPaths = []string{"stderr", logErrorFilePath}

	// Add caller and stack trace to log output
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.StacktraceKey = "stacktrace"

	// Customize time format
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	Log, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}

	zap.ReplaceGlobals(Log) // Replace global logger
}

// Log methods for different levels
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

// WithContext adds context fields to the logger
func WithContext(fields ...zap.Field) *zap.Logger {
	return Log.With(fields...)
}

func Sync() error {
	return Log.Sync()
}
