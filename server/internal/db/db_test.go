package db

import (
	"context"
	"testing"

	"server/config"
)

func TestSelfHealPassword_Noop(t *testing.T) {
	ctx := context.Background()

	// Case 1: Empty bootstrap password
	cfg1 := &config.DatabaseConfig{
		User:              "postgres",
		Password:          "password123",
		BootstrapPassword: "",
		Host:              "localhost",
		Port:              "5432",
		DBName:            "lumiliophotos",
		SSL:               "disable",
	}
	if err := SelfHealPassword(ctx, cfg1); err != nil {
		t.Fatalf("expected nil error when BootstrapPassword is empty, got: %v", err)
	}

	// Case 2: Same password and bootstrap password
	cfg2 := &config.DatabaseConfig{
		User:              "postgres",
		Password:          "password123",
		BootstrapPassword: "password123",
		Host:              "localhost",
		Port:              "5432",
		DBName:            "lumiliophotos",
		SSL:               "disable",
	}
	if err := SelfHealPassword(ctx, cfg2); err != nil {
		t.Fatalf("expected nil error when passwords match, got: %v", err)
	}
}

func TestSelfHealPassword_ConnectionRefusedOrInvalidHost(t *testing.T) {
	ctx := context.Background()

	// If the database host is invalid, pgx.Connect should fail with a dial error or lookup error,
	// which is NOT a password authentication failure. In this case, SelfHealPassword should
	// exit early returning nil (it doesn't alter any user).
	cfg := &config.DatabaseConfig{
		User:              "postgres",
		Password:          "different_rotated_password",
		BootstrapPassword: "postgres",
		Host:              "invalid.host.domain.xyz.invalid",
		Port:              "5432",
		DBName:            "lumiliophotos",
		SSL:               "disable",
	}

	if err := SelfHealPassword(ctx, cfg); err != nil {
		t.Fatalf("expected nil error (early exit on non-auth failure), got: %v", err)
	}
}
