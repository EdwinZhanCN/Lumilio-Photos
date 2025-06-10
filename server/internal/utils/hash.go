package utils

import (
	"fmt"
	"io"
	"os"

	"github.com/zeebo/blake3"
)

const (
	// BufferSize defines the chunk size for reading files during hashing
	// 4MB is a good compromise between memory usage and performance
	BufferSize = 4 * 1024 * 1024
)

// CalculateFileHash computes the BLAKE3 hash of a file and returns it as a hex string
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer file.Close()

	// Create a new BLAKE3 hasher
	hasher := blake3.New()

	// Create a buffer for reading the file in chunks
	buffer := make([]byte, BufferSize)

	// Read the file in chunks and update the hash
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("error reading file during hashing: %w", err)
		}

		if bytesRead == 0 {
			break
		}

		// Update the hash with the bytes read
		_, err = hasher.Write(buffer[:bytesRead])
		if err != nil {
			return "", fmt.Errorf("error updating hash: %w", err)
		}
	}

	// Get the final hash as a hex string
	hashSum := hasher.Sum(nil)
	hashHex := fmt.Sprintf("%x", hashSum)

	return hashHex, nil
}

// VerifyHash checks if the provided hash matches the calculated hash for a file
func VerifyHash(filePath string, expectedHash string) (bool, error) {
	calculatedHash, err := CalculateFileHash(filePath)
	if err != nil {
		return false, err
	}

	return calculatedHash == expectedHash, nil
}