package logging

import (
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
	defaultLogLevel        = "info"
	defaultLogDir          = "server/logs"
	defaultConsoleFormat   = "console"
	defaultFileFormat      = "json"
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

func LoadConfig(defaultLevel string) Config {
	level := strings.TrimSpace(defaultLevel)
	if level == "" {
		level = defaultLogLevel
	}
	if envLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL")); envLevel != "" {
		level = envLevel
	}

	logDir := strings.TrimSpace(os.Getenv("LOG_DIR"))
	if logDir == "" {
		logDir = defaultLogDir
	}

	consoleFormat := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT_CONSOLE")))
	if consoleFormat == "" {
		consoleFormat = defaultConsoleFormat
	}

	fileFormat := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT_FILE")))
	if fileFormat == "" {
		fileFormat = defaultFileFormat
	}

	return Config{
		Level:         level,
		LogDir:        logDir,
		ConsoleFormat: consoleFormat,
		FileFormat:    fileFormat,
		Development:   strings.EqualFold(strings.TrimSpace(os.Getenv("SERVER_ENV")), "development"),
	}
}

func NewLogger(cfg Config) (*Runtime, error) {
	if strings.TrimSpace(cfg.Level) == "" {
		cfg.Level = defaultLogLevel
	}
	if strings.TrimSpace(cfg.LogDir) == "" {
		cfg.LogDir = defaultLogDir
	}
	if strings.TrimSpace(cfg.ConsoleFormat) == "" {
		cfg.ConsoleFormat = defaultConsoleFormat
	}
	if strings.TrimSpace(cfg.FileFormat) == "" {
		cfg.FileFormat = defaultFileFormat
	}

	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, err
	}

	atomicLevel := zap.NewAtomicLevel()
	if err := atomicLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		if err := atomicLevel.UnmarshalText([]byte(defaultLogLevel)); err != nil {
			return nil, err
		}
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
		strings.Contains(message, "inappropriate ioctl for device")
}
