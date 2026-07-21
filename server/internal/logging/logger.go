package logging

import (
	"errors"
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
	globalSecurityLog      = "security.log"
)

type Config struct {
	Level         string
	LogDir        string
	ConsoleFormat string
	FileFormat    string
	Development   bool
}

type Runtime struct {
	logger         *zap.Logger
	securityLogger *zap.Logger
	riverLogger    *slog.Logger
}

func NewLogger(cfg Config) (*Runtime, error) {
	if strings.TrimSpace(cfg.Level) == "" || strings.TrimSpace(cfg.LogDir) == "" || strings.TrimSpace(cfg.ConsoleFormat) == "" || strings.TrimSpace(cfg.FileFormat) == "" {
		return nil, fmt.Errorf("logging config must be complete")
	}

	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, err
	}
	securityPath := filepath.Join(cfg.LogDir, globalSecurityLog)
	if err := prepareSecurityLog(securityPath); err != nil {
		return nil, fmt.Errorf("prepare security log: %w", err)
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
	security := zap.New(
		zapcore.NewCore(
			newEncoder("json", false, true),
			zapcore.AddSync(newRollingWriter(securityPath)),
			zap.LevelEnablerFunc(func(level zapcore.Level) bool { return level >= zapcore.InfoLevel }),
		),
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)

	return &Runtime{
		logger:         root,
		securityLogger: security,
		riverLogger:    slog.New(NewSlogZapHandler(root.With(zap.String("component", "river")))),
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

// Security returns a logger whose events are isolated to security.log. Callers
// must never attach secrets other than the explicitly issued break-glass
// temporary password.
func (r *Runtime) Security() *zap.Logger {
	if r == nil || r.securityLogger == nil {
		return zap.NewNop()
	}
	return r.securityLogger.Named("security").With(zap.String("component", "security"))
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
	var syncErrors []error
	for _, logger := range []*zap.Logger{r.logger, r.securityLogger} {
		if logger == nil {
			continue
		}
		if err := logger.Sync(); err != nil && !ignorableSyncError(err) {
			syncErrors = append(syncErrors, err)
		}
	}
	return errors.Join(syncErrors...)
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

// newRollingWriter returns the concrete *lumberjack.Logger rather than an
// io.Writer: lumberjack holds the file open, so a caller that cannot reach
// Close leaks a handle for the process's lifetime.
func newRollingWriter(path string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    defaultMaxSizeMB,
		MaxBackups: defaultMaxBackups,
		MaxAge:     defaultMaxAgeDays,
		Compress:   defaultCompressBackups,
	}
}

func prepareSecurityLog(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
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
