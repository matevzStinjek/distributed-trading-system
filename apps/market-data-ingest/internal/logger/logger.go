package logger

import (
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around zap.Logger to provide a consistent interface
type Logger struct {
	*zap.Logger
}

// With creates a new Logger with additional fields
func (l *Logger) With(fields ...zapcore.Field) *Logger {
	return &Logger{
		Logger: l.Logger.With(fields...),
	}
}

// Component adds a component field to the logger
func (l *Logger) Component(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("component", component)),
	}
}

// New creates a new logger based on the environment configuration
func New() *Logger {
	// Default to info level
	logLevel := getLogLevelFromEnv()

	// Configure encoder based on LOG_FORMAT
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "time"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeDuration = zapcore.StringDurationEncoder
	encoderCfg.CallerKey = "caller"
	encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder

	// Choose encoder based on format
	var encoder zapcore.Encoder
	switch strings.ToLower(os.Getenv("LOG_FORMAT")) {
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	default:
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	// Create core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stderr),
		logLevel,
	)

	// Create logger with caller info
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{
		Logger: zapLogger,
	}
}

// getLogLevelFromEnv determines the log level from environment variables
func getLogLevelFromEnv() zapcore.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Field creation helpers
func String(key, val string) zapcore.Field {
	return zap.String(key, val)
}

func Int(key string, val int) zapcore.Field {
	return zap.Int(key, val)
}

func Float64(key string, val float64) zapcore.Field {
	return zap.Float64(key, val)
}

func Bool(key string, val bool) zapcore.Field {
	return zap.Bool(key, val)
}

func Error(err error) zapcore.Field {
	return zap.Error(err)
}

func Any(key string, val interface{}) zapcore.Field {
	return zap.Any(key, val)
}

func Int64(key string, val int64) zapcore.Field {
	return zap.Int64(key, val)
}

func Duration(key string, val time.Duration) zapcore.Field {
	return zap.Duration(key, val)
}
