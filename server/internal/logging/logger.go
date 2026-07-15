package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultMaxSizeMB       = 50
	defaultMaxBackups      = 5
	defaultMaxAgeDays      = 14
	defaultCompressBackups = true
	globalApplicationLog   = "app.log"
	globalErrorLog         = "error.log"
)

type Config struct {
	Level         string
	LogDir        string
	ConsoleFormat string
	FileFormat    string
	Development   bool
}

type Runtime struct {
	logger      *zap.Logger
	riverLogger *slog.Logger
}

func NewLogger(cfg Config) (*Runtime, error) {
	if strings.TrimSpace(cfg.Level) == "" || strings.TrimSpace(cfg.LogDir) == "" || strings.TrimSpace(cfg.ConsoleFormat) == "" || strings.TrimSpace(cfg.FileFormat) == "" {
		return nil, fmt.Errorf("logging config must be complete")
	}

	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, err
	}

	atomicLevel := zap.NewAtomicLevel()
	if err := atomicLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, err
	}

	consoleEncoder := newEncoder(cfg.ConsoleFormat, cfg.Development, false)
	fileEncoder := newEncoder(cfg.FileFormat, cfg.Development, true)

	consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), atomicLevel)
	appCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(newRollingWriter(filepath.Join(cfg.LogDir, globalApplicationLog))),
		atomicLevel,
	)
	errorCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(newRollingWriter(filepath.Join(cfg.LogDir, globalErrorLog))),
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level >= zapcore.WarnLevel && level >= atomicLevel.Level()
		}),
	)

	root := zap.New(
		zapcore.NewTee(consoleCore, appCore, errorCore),
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.ErrorOutput(zapcore.AddSync(newRollingWriter(filepath.Join(cfg.LogDir, globalErrorLog)))),
	)

	return &Runtime{
		logger:      root,
		riverLogger: slog.New(NewSlogZapHandler(root.With(zap.String("component", "river")))),
	}, nil
}

func (r *Runtime) Logger() *zap.Logger {
	if r == nil || r.logger == nil {
		return zap.NewNop()
	}
	return r.logger
}

func (r *Runtime) Named(component string) *zap.Logger {
	component = strings.TrimSpace(component)
	if component == "" {
		return r.Logger()
	}
	return r.Logger().Named(component).With(zap.String("component", component))
}

func (r *Runtime) RiverLogger() *slog.Logger {
	if r == nil || r.riverLogger == nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return r.riverLogger
}

func (r *Runtime) Sync() error {
	if r == nil || r.logger == nil {
		return nil
	}
	err := r.logger.Sync()
	if err == nil {
		return nil
	}
	if ignorableSyncError(err) {
		return nil
	}
	return err
}

func newEncoder(format string, development bool, fileOutput bool) zapcore.Encoder {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeDuration = zapcore.MillisDurationEncoder
	if development && !fileOutput {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
		return zapcore.NewConsoleEncoder(encoderCfg)
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "console":
		encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
		return zapcore.NewConsoleEncoder(encoderCfg)
	default:
		return zapcore.NewJSONEncoder(encoderCfg)
	}
}

func newRollingWriter(path string) io.Writer {
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    defaultMaxSizeMB,
		MaxBackups: defaultMaxBackups,
		MaxAge:     defaultMaxAgeDays,
		Compress:   defaultCompressBackups,
	}
}

func ignorableSyncError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "bad file descriptor") ||
		strings.Contains(message, "inappropriate ioctl for device") ||
		strings.Contains(message, "invalid argument")
}
