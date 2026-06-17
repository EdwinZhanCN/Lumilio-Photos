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
	"server/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

type reservingCredentialRotator interface {
	ReserveRotation(ctx context.Context) (DBCredentialRotator, func(), error)
}

// SetupRequest carries the first-run setup payload submitted from the web wizard.
type SetupRequest struct{}

// SetupStatus reports whether the database credential rotation has completed.
type SetupStatus struct {
	Initialized                  bool
	DatabaseInitialized          bool
	AdminInitialized             bool
	PrimaryRepositoryInitialized bool
	NextRegistrationRole         string
	RepositoryDefaults           *RepositoryDefaults
}

// RepositoryDefaults is the setup-wizard view of the storage-owned repository
// defaults plus the immutable default root (the storage root).
type RepositoryDefaults struct {
	DefaultRoot       string
	Strategy          string
	DuplicateHandling string
}

// repositoryDefaultsReader reads the storage-owned repository defaults.
type repositoryDefaultsReader interface {
	GetRepositoryDefaults(ctx context.Context) (storage.RepoDefaults, error)
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
	dbConfig     config.DatabaseConfig
	rotator      DBCredentialRotator
	secretPath   string
	bootstrap    BootstrapService
	repoDefaults repositoryDefaultsReader
	storageRoot  string
}

// NewSetupService wires a setup service using the live (temporary) database
// configuration as the rotation credential and the configured on-disk paths.
func NewSetupService(dbConfig config.DatabaseConfig) *SetupService {
	return &SetupService{
		dbConfig:   dbConfig,
		rotator:    &pgxCredentialRotator{cfg: dbConfig},
		secretPath: strings.TrimSpace(dbConfig.PasswordFile),
	}
}

// NewSetupServiceWithPool wires setup to the already-open application pool. This
// lets setup recover when the bootstrap password in TOML/env is stale but the
// running server still owns a valid database session.
func NewSetupServiceWithPool(dbConfig config.DatabaseConfig, pool *pgxpool.Pool, bootstrap BootstrapService, repoDefaults repositoryDefaultsReader, storageRoot string) *SetupService {
	rotator := DBCredentialRotator(&pgxCredentialRotator{cfg: dbConfig})
	if pool != nil {
		rotator = &pgxPoolCredentialRotator{pool: pool}
	}
	return &SetupService{
		dbConfig:     dbConfig,
		rotator:      rotator,
		secretPath:   strings.TrimSpace(dbConfig.PasswordFile),
		bootstrap:    bootstrap,
		repoDefaults: repoDefaults,
		storageRoot:  strings.TrimSpace(storageRoot),
	}
}

// Status reports whether the rotated database password secret already exists on
// disk. The web frontend uses this to decide whether first-run setup still
// needs to rotate the temporary bootstrap credential.
func (s *SetupService) Status(ctx context.Context) (SetupStatus, error) {
	status := SetupStatus{}

	if s.bootstrap != nil {
		phase, err := s.bootstrap.Phase(ctx)
		if err != nil {
			return SetupStatus{}, fmt.Errorf("load bootstrap phase: %w", err)
		}
		status.DatabaseInitialized = phase != BootstrapPhaseFresh
		status.AdminInitialized = phase == BootstrapPhaseAdminCreated || phase == BootstrapPhaseReady
		status.PrimaryRepositoryInitialized = phase == BootstrapPhaseReady
		status.Initialized = phase == BootstrapPhaseReady
	}

	// The first registration (no admin yet) becomes the admin; subsequent ones
	// are regular users. Folds the former /auth/bootstrap-status semantics.
	if status.AdminInitialized {
		status.NextRegistrationRole = string(UserRoleUser)
	} else {
		status.NextRegistrationRole = string(UserRoleAdmin)
	}

	if s.repoDefaults != nil {
		defaults, err := s.repoDefaults.GetRepositoryDefaults(ctx)
		if err != nil {
			return SetupStatus{}, fmt.Errorf("load repository defaults: %w", err)
		}
		status.RepositoryDefaults = &RepositoryDefaults{
			DefaultRoot:       s.storageRoot,
			Strategy:          defaults.Strategy,
			DuplicateHandling: defaults.DuplicateHandling,
		}
	}

	return status, nil
}

// Initialize performs first-run bootstrapping. It is idempotent-by-refusal: a
// second call once the password file exists returns ErrSystemAlreadyInitialized.
func (s *SetupService) Initialize(ctx context.Context, _ SetupRequest) (SetupResult, error) {
	setupInitializeMu.Lock()
	defer setupInitializeMu.Unlock()

	// Setup only rotates the database credential; refuse once the rotated secret
	// exists so a second call cannot mint a fresh password. This gate is the
	// password file itself (DB-free), independent of the cached bootstrap phase.
	if s.databaseCredentialRotated() {
		return SetupResult{}, ErrSystemAlreadyInitialized
	}

	rotator := s.rotator
	releaseRotation := func() {}
	if reservingRotator, ok := s.rotator.(reservingCredentialRotator); ok {
		reservedRotator, release, err := reservingRotator.ReserveRotation(ctx)
		if err != nil {
			return SetupResult{}, fmt.Errorf("reserve database rotation connection: %w", err)
		}
		rotator = reservedRotator
		releaseRotation = release
	}
	defer releaseRotation()

	newPassword, err := generateHighEntropyPassword(generatedPasswordLength)
	if err != nil {
		return SetupResult{}, fmt.Errorf("generate database password: %w", err)
	}

	// Persist the new secret with locked-down permissions so only the runtime
	// process can read it. This happens before ALTER USER so a local filesystem
	// failure cannot leave PostgreSQL rotated with no password file on disk.
	if err := writeSecretFile(s.secretPath, newPassword); err != nil {
		return SetupResult{}, fmt.Errorf("persist database secret: %w", err)
	}

	// Rotate the database credential away from the bootstrap password using the
	// initial temporary credential, then terminate that session.
	if err := rotator.RotatePassword(ctx, s.dbConfig.User, newPassword); err != nil {
		_ = removeSecretFileIfMatches(s.secretPath, newPassword)
		return SetupResult{}, fmt.Errorf("rotate database credential: %w", err)
	}

	// Advance the bootstrap phase (fresh → db_rotated) now that the credential
	// gate is satisfied.
	if s.bootstrap != nil {
		if _, err := s.bootstrap.Reconcile(ctx); err != nil {
			return SetupResult{}, fmt.Errorf("reconcile bootstrap phase: %w", err)
		}
	}

	return SetupResult{
		DatabaseUser:   s.dbConfig.User,
		SecretPath:     s.secretPath,
		PasswordLength: len(newPassword),
	}, nil
}

// databaseCredentialRotated reports whether the rotated DB password secret
// already exists on disk (the db_rotated gate).
func (s *SetupService) databaseCredentialRotated() bool {
	data, err := os.ReadFile(s.secretPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) != ""
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

func removeSecretFileIfMatches(path, secret string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) != secret {
		return nil
	}
	return os.Remove(path)
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

// pgxPoolCredentialRotator rotates through an existing application pool. It
// reserves a live connection before setup writes the new password file so the
// pool cannot race by opening a fresh connection with the not-yet-activated
// password.
type pgxPoolCredentialRotator struct {
	pool *pgxpool.Pool
}

func (r *pgxPoolCredentialRotator) ReserveRotation(ctx context.Context) (DBCredentialRotator, func(), error) {
	if r.pool == nil {
		return nil, nil, errors.New("database pool unavailable")
	}
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("acquire live database connection: %w", err)
	}
	return &pgxPoolConnCredentialRotator{conn: conn}, conn.Release, nil
}

func (r *pgxPoolCredentialRotator) RotatePassword(ctx context.Context, username, newPassword string) error {
	reserved, release, err := r.ReserveRotation(ctx)
	if err != nil {
		return err
	}
	defer release()
	return reserved.RotatePassword(ctx, username, newPassword)
}

type pgxPoolConnCredentialRotator struct {
	conn *pgxpool.Conn
}

func (r *pgxPoolConnCredentialRotator) RotatePassword(ctx context.Context, username, newPassword string) error {
	stmt := fmt.Sprintf("ALTER USER %s WITH PASSWORD %s",
		quoteSQLIdentifier(username),
		quoteSQLLiteral(newPassword),
	)
	if _, err := r.conn.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("alter user through live pool: %w", err)
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
