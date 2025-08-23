package db

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"server/config"
	"strings"
)

type MigrationConfig struct {
	DatabaseConfig config.DatabaseConfig
	MigrationsDir  string
}

func NewMigrationConfig(dbConfig config.DatabaseConfig) *MigrationConfig {
	return &MigrationConfig{
		DatabaseConfig: dbConfig,
		MigrationsDir:  "migrations",
	}
}

func (m *MigrationConfig) buildURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&channel_binding=%s&search_path=public",
		m.DatabaseConfig.User,
		m.DatabaseConfig.Password,
		m.DatabaseConfig.Host,
		m.DatabaseConfig.Port,
		m.DatabaseConfig.DBName,
		m.DatabaseConfig.SSL,
		m.DatabaseConfig.ChannelBinding,
	)
}

func resolveWorkDir() string {
	if _, err := os.Stat("atlas.hcl"); err == nil {
		return "."
	}
	if _, err := os.Stat(filepath.Join("server", "atlas.hcl")); err == nil {
		return "server"
	}

	return "."
}

func generateInitialIfEmpty(ctx context.Context, workDir, migrationsDir string) error {
	entries, err := os.ReadDir(filepath.Join(workDir, migrationsDir))
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	if len(entries) > 0 {
		return nil
	}

	if _, err := os.Stat(filepath.Join(workDir, "atlas.hcl")); err != nil {
		log.Printf("No migrations found and no atlas.hcl present; skip auto-generation.")
		return nil
	}
	log.Printf("🆕 No migrations found in %s, generating initial from schema...", migrationsDir)
	cmd := exec.CommandContext(ctx, "atlas", "migrate", "diff", "initial",
		"--config", "file://atlas.hcl",
		"--env", "dev",
	)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *MigrationConfig) apply(ctx context.Context, workDir string) error {
	dirArg := fmt.Sprintf("file://%s", m.MigrationsDir)
	cmd := exec.CommandContext(ctx, "atlas", "migrate", "apply",
		"--url", m.buildURL(),
		"--dir", dirArg,
		"--revisions-schema", "public",
	)
	cmd.Dir = workDir

	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	log.Printf("🚀 Applying migrations... (dir=%s, workDir=%s, revisions_schema=public)", dirArg, workDir)
	err := cmd.Run()

	stdout := strings.TrimSpace(out.String())
	stderr := strings.TrimSpace(errb.String())
	if stdout != "" {
		log.Println(stdout)
	}
	if err != nil {
		if stderr != "" {
			return fmt.Errorf("atlas apply failed: %w\n%s", err, stderr)
		}
		return fmt.Errorf("atlas apply failed: %w", err)
	}
	if stderr != "" {
		log.Println(stderr)
	}
	return nil
}

func (m *MigrationConfig) runRiverMigrations(ctx context.Context) error {
	databaseURL := m.buildURL()
	cmd := exec.CommandContext(ctx, "river", "migrate-up", "--database-url", databaseURL)

	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	log.Printf("🌊 Running River migrations...")
	err := cmd.Run()

	stdout := strings.TrimSpace(out.String())
	stderr := strings.TrimSpace(errb.String())
	if stdout != "" {
		log.Println(stdout)
	}
	if err != nil {
		if stderr != "" {
			return fmt.Errorf("river migrate-up failed: %w\n%s", err, stderr)
		}
		return fmt.Errorf("river migrate-up failed: %w", err)
	}
	if stderr != "" {
		log.Println(stderr)
	}
	return nil
}

func (m *MigrationConfig) RunMigrations(ctx context.Context) error {

	if err := checkAtlasAvailable(ctx); err != nil {
		return fmt.Errorf("Atlas CLI not available: %w", err)
	}

	if err := checkRiverAvailable(ctx); err != nil {
		return fmt.Errorf("River CLI not available: %w", err)
	}

	workDir := resolveWorkDir()
	migrationsPath := filepath.Join(workDir, m.MigrationsDir)
	if err := os.MkdirAll(migrationsPath, 0o755); err != nil {
		return fmt.Errorf("create migrations dir: %w", err)
	}

	if err := generateInitialIfEmpty(ctx, workDir, m.MigrationsDir); err != nil {
		return fmt.Errorf("failed to generate initial migration: %w", err)
	}

	if err := m.apply(ctx, workDir); err != nil {
		return err
	}

	if err := m.runRiverMigrations(ctx); err != nil {
		return err
	}

	log.Printf("✅ Migrations applied successfully.")
	return nil
}

func AutoMigrate(ctx context.Context, dbConfig config.DatabaseConfig) error {
	m := NewMigrationConfig(dbConfig)
	return m.RunMigrations(ctx)
}

func checkAtlasAvailable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "atlas", "version")
	if err := cmd.Run(); err != nil {
		return errors.New("atlas not found in PATH")
	}
	return nil
}

func checkRiverAvailable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "river", "--version")
	if err := cmd.Run(); err != nil {
		return errors.New("river not found in PATH")
	}
	return nil
}
