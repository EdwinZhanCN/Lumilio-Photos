package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"server/config"

	"github.com/jackc/pgx/v5"
)

// ErrSystemAlreadyInitialized is returned when setup is attempted on a system
// whose configuration payload already exists on disk.
var ErrSystemAlreadyInitialized = errors.New("system already initialized")

var setupInitializeMu sync.Mutex

// generatedPasswordLength is the length of the high-entropy database password
// minted during first-run bootstrapping.
const generatedPasswordLength = 32

// passwordAlphabet is an unambiguous alphanumeric alphabet. Keeping the
// generated password free of shell/SQL metacharacters means it is safe to embed
// in a DSN or an ALTER USER literal without surprising quoting behaviour.
const passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// DBCredentialRotator rotates the database role password using the initial
// temporary credential. Implementations open a short-lived connection, apply the
// new password with a native user-alteration command, and terminate the session.
type DBCredentialRotator interface {
	RotatePassword(ctx context.Context, username, newPassword string) error
}

// SetupRequest carries the first-run setup payload submitted from the web wizard.
type SetupRequest struct{}

// SetupStatus reports whether the database credential rotation has completed.
type SetupStatus struct {
	Initialized bool `json:"initialized"`
}

// SetupResult summarises a completed initialization.
type SetupResult struct {
	DatabaseUser   string `json:"database_user"`
	SecretPath     string `json:"secret_path"`
	PasswordLength int    `json:"password_length"`
}

// SetupService implements the zero-config first-run bootstrapping flow: it
// rotates the database credential away from the temporary bootstrap password,
// and persists the new secret with locked-down permissions.
type SetupService struct {
	dbConfig   config.DatabaseConfig
	rotator    DBCredentialRotator
	secretPath string
}

// NewSetupService wires a setup service using the live (temporary) database
// configuration as the rotation credential and the configured on-disk paths.
func NewSetupService(dbConfig config.DatabaseConfig) *SetupService {
	return &SetupService{
		dbConfig:   dbConfig,
		rotator:    &pgxCredentialRotator{cfg: dbConfig},
		secretPath: config.ResolveDBPasswordFilePath(dbConfig),
	}
}

// Status reports whether the rotated database password secret already exists on
// disk. The web frontend uses this to decide whether first-run setup still
// needs to rotate the temporary bootstrap credential.
func (s *SetupService) Status(_ context.Context) (SetupStatus, error) {
	data, err := os.ReadFile(s.secretPath)
	if err == nil {
		return SetupStatus{Initialized: strings.TrimSpace(string(data)) != ""}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return SetupStatus{}, fmt.Errorf("read database password file %s: %w", s.secretPath, err)
	}
	return SetupStatus{Initialized: false}, nil
}

// Initialize performs first-run bootstrapping. It is idempotent-by-refusal: a
// second call once the password file exists returns ErrSystemAlreadyInitialized.
func (s *SetupService) Initialize(ctx context.Context, _ SetupRequest) (SetupResult, error) {
	setupInitializeMu.Lock()
	defer setupInitializeMu.Unlock()

	status, err := s.Status(ctx)
	if err != nil {
		return SetupResult{}, err
	}
	if status.Initialized {
		return SetupResult{}, ErrSystemAlreadyInitialized
	}

	newPassword, err := generateHighEntropyPassword(generatedPasswordLength)
	if err != nil {
		return SetupResult{}, fmt.Errorf("generate database password: %w", err)
	}

	// Rotate the database credential away from the bootstrap password using the
	// initial temporary credential, then terminate that session.
	if err := s.rotator.RotatePassword(ctx, s.dbConfig.User, newPassword); err != nil {
		return SetupResult{}, fmt.Errorf("rotate database credential: %w", err)
	}

	// Persist the new secret with locked-down permissions so only the runtime
	// process can read it.
	if err := writeSecretFile(s.secretPath, newPassword); err != nil {
		return SetupResult{}, fmt.Errorf("persist database secret: %w", err)
	}

	return SetupResult{
		DatabaseUser:   s.dbConfig.User,
		SecretPath:     s.secretPath,
		PasswordLength: len(newPassword),
	}, nil
}

// writeSecretFile writes a secret to disk and forces 0600 permissions so the
// value is readable only by the runtime process owner, regardless of umask.
func writeSecretFile(path, secret string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create secret dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return fmt.Errorf("write secret %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("lock secret permissions %s: %w", path, err)
	}
	return nil
}

// generateHighEntropyPassword returns a cryptographically secure random string
// of the requested length drawn from an unambiguous alphanumeric alphabet.
func generateHighEntropyPassword(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid password length %d", length)
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	alphabetLen := byte(len(passwordAlphabet))
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = passwordAlphabet[b%alphabetLen]
	}
	return string(out), nil
}

// pgxCredentialRotator rotates the password using a short-lived pgx connection
// authenticated with the initial temporary credential.
type pgxCredentialRotator struct {
	cfg config.DatabaseConfig
}

func (r *pgxCredentialRotator) RotatePassword(ctx context.Context, username, newPassword string) error {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		r.cfg.User,
		r.cfg.Password,
		r.cfg.Host,
		r.cfg.Port,
		r.cfg.DBName,
		r.cfg.SSL,
	)

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect with temporary credential: %w", err)
	}
	defer conn.Close(ctx)

	stmt := fmt.Sprintf("ALTER USER %s WITH PASSWORD %s",
		quoteSQLIdentifier(username),
		quoteSQLLiteral(newPassword),
	)
	if _, err := conn.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("alter user: %w", err)
	}
	return nil
}

// quoteSQLIdentifier safely double-quotes a SQL identifier (e.g. a role name).
func quoteSQLIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// quoteSQLLiteral safely single-quotes a SQL string literal.
func quoteSQLLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}
