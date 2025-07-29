package exif

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// IsExifToolAvailable checks if exiftool is available in the system
func IsExifToolAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "exiftool", "-ver")
	return cmd.Run() == nil
}

// GetExifToolVersion returns the version of exiftool if available
func GetExifToolVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "exiftool", "-ver")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// ValidateExifToolInstallation performs a comprehensive check of exiftool installation
func ValidateExifToolInstallation() error {
	// Check if exiftool is in PATH
	if _, err := exec.LookPath("exiftool"); err != nil {
		return err
	}

	// Check if exiftool responds to version command
	if !IsExifToolAvailable() {
		return fmt.Errorf("exiftool is installed but not responding to version command")
	}

	return nil
}

// checkExifToolSupport checks if exiftool supports required features
func checkExifToolSupport() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test JSON output support
	cmd := exec.CommandContext(ctx, "exiftool", "-j", "-ver")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exiftool does not support JSON output: %w", err)
	}

	// Test stdin reading support
	cmd = exec.CommandContext(ctx, "exiftool", "-j", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("exiftool stdin pipe not available: %w", err)
	}
	stdin.Close()

	return nil
}
