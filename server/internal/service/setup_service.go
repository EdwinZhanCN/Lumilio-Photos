package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/config"

	"github.com/jackc/pgx/v5"
	"github.com/pelletier/go-toml/v2"
)

// ErrSystemAlreadyInitialized is returned when setup is attempted on a system
// whose configuration payload already exists on disk.
var ErrSystemAlreadyInitialized = errors.New("system already initialized")

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
type SetupRequest struct {
	SiteName      string
	AdminUsername string
}

// SetupStatus reports whether the system configuration payload exists on disk.
type SetupStatus struct {
	Initialized bool `json:"initialized"`
}

// SetupResult summarises a completed initialization.
type SetupResult struct {
	SiteName       string `json:"site_name"`
	AdminUsername  string `json:"admin_username"`
	DatabaseUser   string `json:"database_user"`
	SecretPath     string `json:"secret_path"`
	ConfigPath     string `json:"config_path"`
	PasswordLength int    `json:"password_length"`
}

// systemConfigFile is the non-sensitive metadata persisted as structured TOML in
// the application's data directory once setup completes.
type systemConfigFile struct {
	System struct {
		SiteName      string `toml:"site_name"`
		AdminUsername string `toml:"admin_username"`
		Initialized   bool   `toml:"initialized"`
		InitializedAt string `toml:"initialized_at"`
	} `toml:"system"`
	Database struct {
		User    string `toml:"user"`
		Rotated bool   `toml:"rotated"`
	} `toml:"database"`
}

// SetupService implements the zero-config first-run bootstrapping flow: it
// rotates the database credential away from the temporary bootstrap password,
// persists the new secret with locked-down permissions, and records
// non-sensitive metadata as TOML.
type SetupService struct {
	dbConfig   config.DatabaseConfig
	rotator    DBCredentialRotator
	secretPath string
	configPath string
	now        func() time.Time
}

// NewSetupService wires a setup service using the live (temporary) database
// configuration as the rotation credential and the configured on-disk paths.
func NewSetupService(dbConfig config.DatabaseConfig) *SetupService {
	return &SetupService{
		dbConfig:   dbConfig,
		rotator:    &pgxCredentialRotator{cfg: dbConfig},
		secretPath: config.DBPasswordFilePath(),
		configPath: config.SystemConfigFilePath(),
		now:        time.Now,
	}
}

// Status reports whether the system configuration payload already exists on
// disk. The web frontend uses this to decide whether to route to the setup
// wizard or the normal application.
func (s *SetupService) Status(_ context.Context) (SetupStatus, error) {
	if _, err := os.Stat(s.configPath); err == nil {
		return SetupStatus{Initialized: true}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return SetupStatus{}, fmt.Errorf("stat system config %s: %w", s.configPath, err)
	}
	return SetupStatus{Initialized: false}, nil
}

// Initialize performs first-run bootstrapping. It is idempotent-by-refusal: a
// second call once the config payload exists returns ErrSystemAlreadyInitialized.
func (s *SetupService) Initialize(ctx context.Context, req SetupRequest) (SetupResult, error) {
	status, err := s.Status(ctx)
	if err != nil {
		return SetupResult{}, err
	}
	if status.Initialized {
		return SetupResult{}, ErrSystemAlreadyInitialized
	}

	siteName := strings.TrimSpace(req.SiteName)
	if siteName == "" {
		siteName = "Lumilio Photos"
	}
	adminUsername := strings.TrimSpace(req.AdminUsername)

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

	// Record non-sensitive metadata as structured TOML in the data directory.
	if err := s.writeSystemConfig(siteName, adminUsername); err != nil {
		return SetupResult{}, fmt.Errorf("persist system config: %w", err)
	}

	return SetupResult{
		SiteName:       siteName,
		AdminUsername:  adminUsername,
		DatabaseUser:   s.dbConfig.User,
		SecretPath:     s.secretPath,
		ConfigPath:     s.configPath,
		PasswordLength: len(newPassword),
	}, nil
}

func (s *SetupService) writeSystemConfig(siteName, adminUsername string) error {
	var file systemConfigFile
	file.System.SiteName = siteName
	file.System.AdminUsername = adminUsername
	file.System.Initialized = true
	file.System.InitializedAt = s.now().UTC().Format(time.RFC3339)
	file.Database.User = s.dbConfig.User
	file.Database.Rotated = true

	data, err := toml.Marshal(file)
	if err != nil {
		return fmt.Errorf("marshal system config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(s.configPath, data, 0o600); err != nil {
		return fmt.Errorf("write system config %s: %w", s.configPath, err)
	}
	return nil
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
