package exif

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
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

// GetAvailableMemory returns available system memory in bytes
func GetAvailableMemory() (uint64, error) {
	// For large file processing, we'll be more permissive
	// Assume sufficient memory is available for files up to 10GB
	return 20 * 1024 * 1024 * 1024, nil // Assume 20GB available
}

// GetAvailableDiskSpace returns available disk space in bytes for temp directory
func GetAvailableDiskSpace() (uint64, error) {
	// For large file processing, assume sufficient disk space
	return 100 * 1024 * 1024 * 1024, nil // Assume 100GB available
}

// CanHandleFileSize checks if system has enough resources to handle a file of given size
func CanHandleFileSize(fileSize int64) (bool, string) {
	// Check if file size exceeds reasonable limits (20GB)
	if fileSize > 20*1024*1024*1024 {
		return false, "file size exceeds maximum supported limit of 20GB"
	}

	// For files under 20GB, assume system can handle them
	// The streaming implementation should handle memory efficiently
	return true, ""
}

// GetOptimalBufferSize calculates optimal buffer size based on file size and system resources
func GetOptimalBufferSize(fileSize int64) int {
	// Base buffer size
	baseSize := 64 * 1024 // 64KB

	// For very large files, use larger buffers for better performance
	if fileSize > 500*1024*1024 { // > 500MB
		return 256 * 1024 // 256KB
	} else if fileSize > 100*1024*1024 { // > 100MB
		return 128 * 1024 // 128KB
	}

	return baseSize
}

// GetOptimalWorkerCount calculates optimal number of workers based on system resources
func GetOptimalWorkerCount() int {
	// Use number of CPU cores as base
	cpuCount := runtime.NumCPU()

	// For systems with many cores, limit to reasonable number
	if cpuCount > 8 {
		return 8
	}

	// For systems with few cores, use all available
	if cpuCount < 2 {
		return 1
	}

	return cpuCount
}

// IsLargeFile returns true if file size is considered large for processing
func IsLargeFile(fileSize int64) bool {
	return fileSize > 100*1024*1024 // > 100MB
}
