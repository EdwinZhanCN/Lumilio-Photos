package logging

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	repositoryLogsDir    = ".lumilio/logs"
	repositoryOpsLogName = "operations.log"
	repositoryErrLogName = "error.log"
)

type RepositoryAuditProvider interface {
	ForPath(repoPath string) RepositoryAuditLogger
}

type RepositoryAuditLogger interface {
	Operation(operation string, fields ...zap.Field)
	Error(operation string, err error, fields ...zap.Field)
}

type repositoryAuditProvider struct {
	mu       sync.Mutex
	enc      zapcore.EncoderConfig
	cache    map[string]*repositoryAuditLogger
	base     *zap.Logger
	fileMode os.FileMode
}

type repositoryAuditLogger struct {
	repoPath    string
	operations  *zap.Logger
	errorLogger *zap.Logger
}

type noopRepositoryAuditLogger struct{}

func NewRepositoryAuditProvider(baseLogger *zap.Logger) RepositoryAuditProvider {
	if baseLogger == nil {
		baseLogger = zap.NewNop()
	}
	enc := zap.NewProductionEncoderConfig()
	enc.TimeKey = "ts"
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	enc.EncodeDuration = zapcore.MillisDurationEncoder

	return &repositoryAuditProvider{
		enc:      enc,
		cache:    make(map[string]*repositoryAuditLogger),
		base:     baseLogger.With(zap.String("component", "repo_audit")),
		fileMode: 0644,
	}
}

func (p *repositoryAuditProvider) ForPath(repoPath string) RepositoryAuditLogger {
	cleanPath, err := filepath.Abs(filepath.Clean(strings.TrimSpace(repoPath)))
	if err != nil {
		return noopRepositoryAuditLogger{}
	}

	logsDir := filepath.Join(cleanPath, repositoryLogsDir)
	if _, err := os.Stat(logsDir); err != nil {
		return noopRepositoryAuditLogger{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existing, ok := p.cache[cleanPath]; ok {
		return existing
	}

	opsPath := filepath.Join(logsDir, repositoryOpsLogName)
	errPath := filepath.Join(logsDir, repositoryErrLogName)
	if err := ensureFile(opsPath, p.fileMode); err != nil {
		p.base.Warn("failed to prepare repository operations log", zap.String("repository_path", cleanPath), zap.Error(err))
		return noopRepositoryAuditLogger{}
	}
	if err := ensureFile(errPath, p.fileMode); err != nil {
		p.base.Warn("failed to prepare repository error log", zap.String("repository_path", cleanPath), zap.Error(err))
		return noopRepositoryAuditLogger{}
	}

	opsLogger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(p.enc),
		zapcore.AddSync(newRollingWriter(opsPath)),
		zapcore.InfoLevel,
	))
	errLogger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(p.enc),
		zapcore.AddSync(newRollingWriter(errPath)),
		zapcore.WarnLevel,
	))

	logger := &repositoryAuditLogger{
		repoPath:    cleanPath,
		operations:  opsLogger,
		errorLogger: errLogger,
	}
	p.cache[cleanPath] = logger
	return logger
}

func (l *repositoryAuditLogger) Operation(operation string, fields ...zap.Field) {
	if l == nil || l.operations == nil {
		return
	}
	allFields := append([]zap.Field{
		zap.String("repository_path", l.repoPath),
		zap.String("operation", strings.TrimSpace(operation)),
		zap.String("result", "ok"),
	}, fields...)
	l.operations.Info(operation, allFields...)
}

func (l *repositoryAuditLogger) Error(operation string, err error, fields ...zap.Field) {
	if l == nil || l.errorLogger == nil {
		return
	}
	allFields := append([]zap.Field{
		zap.String("repository_path", l.repoPath),
		zap.String("operation", strings.TrimSpace(operation)),
		zap.String("result", "error"),
	}, fields...)
	if err != nil {
		allFields = append(allFields, zap.Error(err))
	}
	l.errorLogger.Warn(operation, allFields...)
}

func (noopRepositoryAuditLogger) Operation(string, ...zap.Field) {}

func (noopRepositoryAuditLogger) Error(string, error, ...zap.Field) {}

func ensureFile(path string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE, mode)
	if err != nil {
		return err
	}
	return file.Close()
}
