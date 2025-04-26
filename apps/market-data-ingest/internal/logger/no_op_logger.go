package logger

import (
	"io/ioutil"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewNoOpLogger returns a logger that doesn't produce output
// which is useful for testing
func NewNoOpLogger() *Logger {
	// Create a no-op core that discards all logs
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "time",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(ioutil.Discard),
		zapcore.InfoLevel,
	)

	// Create a new zap logger with the no-op core
	zapLogger := zap.New(core)

	return &Logger{
		Logger: zapLogger,
	}
}
