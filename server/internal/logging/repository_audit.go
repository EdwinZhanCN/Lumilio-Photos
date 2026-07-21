package logging

import (
	"io"
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

	// Close releases the per-repository log files this provider opened. The
	// provider caches one logger per repository path and never evicts, so
	// without this every repository ever touched holds two open file handles
	// for the process's lifetime. On Windows those handles also make the
	// repository directory undeletable.
	Close() error
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
	verbose  bool
}

type repositoryAuditLogger struct {
	repoPath    string
	operations  *zap.Logger
	errorLogger *zap.Logger
	writers     []io.Closer
}

type noopRepositoryAuditLogger struct{}

// NoopRepositoryAuditLogger is the audit sink for a value that has no provider
// configured. Building a throwaway provider in that case would open per-repository
// log files that nothing can ever close, once per call.
func NoopRepositoryAuditLogger() RepositoryAuditLogger {
	return noopRepositoryAuditLogger{}
}

func NewRepositoryAuditProvider(baseLogger *zap.Logger, verbose bool) RepositoryAuditProvider {
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
		verbose:  verbose,
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

	minOperationLevel := zapcore.InfoLevel
	if p.verbose {
		minOperationLevel = zapcore.DebugLevel
	}
	opsWriter := newRollingWriter(opsPath)
	errWriter := newRollingWriter(errPath)
	opsLogger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(p.enc),
		zapcore.AddSync(opsWriter),
		zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return l >= minOperationLevel
		}),
	))
	errLogger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(p.enc),
		zapcore.AddSync(errWriter),
		zapcore.WarnLevel,
	))

	logger := &repositoryAuditLogger{
		repoPath:    cleanPath,
		operations:  opsLogger,
		errorLogger: errLogger,
		writers:     []io.Closer{opsWriter, errWriter},
	}
	p.cache[cleanPath] = logger
	return logger
}

// Close closes every cached repository logger's files and empties the cache, so
// a provider that is closed and used again reopens rather than writing to a
// closed file.
func (p *repositoryAuditProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for path, logger := range p.cache {
		for _, writer := range logger.writers {
			if err := writer.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		delete(p.cache, path)
	}
	return firstErr
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
	l.operations.Debug(operation, allFields...)
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
