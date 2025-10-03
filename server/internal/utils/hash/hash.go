package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/zeebo/blake3"
)

const (
	// ChunkSize for quick hash: read first and last chunks
	QuickHashChunkSize = 1 * 1024 * 1024 // 1 MB

	// Threshold for using quick hash vs full hash (100 MB)
	QuickHashThreshold = 100 * 1024 * 1024
)

// HashAlgorithm defines the hashing algorithm to use
type HashAlgorithm string

const (
	AlgorithmBLAKE3 HashAlgorithm = "blake3"
	AlgorithmSHA256 HashAlgorithm = "sha256"
)

// HashResult contains hash information
type HashResult struct {
	Algorithm HashAlgorithm `json:"algorithm"`
	Hash      string        `json:"hash"`
	IsQuick   bool          `json:"is_quick"` // true if quick hash was used
	FileSize  int64         `json:"file_size"`
}

// CalculateFileHash calculates the hash of a file using the specified algorithm
// For large files (>100MB), it can optionally use a quick hash strategy
func CalculateFileHash(filePath string, algorithm HashAlgorithm, useQuickForLarge bool) (*HashResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	// Determine if we should use quick hash
	useQuick := useQuickForLarge && fileSize > QuickHashThreshold

	var hash string
	if useQuick {
		hash, err = calculateQuickHash(file, fileSize, algorithm)
	} else {
		hash, err = calculateFullHash(file, algorithm)
	}

	if err != nil {
		return nil, err
	}

	return &HashResult{
		Algorithm: algorithm,
		Hash:      hash,
		IsQuick:   useQuick,
		FileSize:  fileSize,
	}, nil
}

// CalculateBLAKE3 calculates BLAKE3 hash of a file
func CalculateBLAKE3(filePath string) (string, error) {
	result, err := CalculateFileHash(filePath, AlgorithmBLAKE3, false)
	if err != nil {
		return "", err
	}
	return result.Hash, nil
}

// CalculateSHA256 calculates SHA256 hash of a file
func CalculateSHA256(filePath string) (string, error) {
	result, err := CalculateFileHash(filePath, AlgorithmSHA256, false)
	if err != nil {
		return "", err
	}
	return result.Hash, nil
}

// CalculateQuickBLAKE3 calculates quick BLAKE3 hash for large files
func CalculateQuickBLAKE3(filePath string) (string, error) {
	result, err := CalculateFileHash(filePath, AlgorithmBLAKE3, true)
	if err != nil {
		return "", err
	}
	return result.Hash, nil
}

// CalculateQuickSHA256 calculates quick SHA256 hash for large files
func CalculateQuickSHA256(filePath string) (string, error) {
	result, err := CalculateFileHash(filePath, AlgorithmSHA256, true)
	if err != nil {
		return "", err
	}
	return result.Hash, nil
}

// calculateFullHash calculates the full hash of a file
func calculateFullHash(reader io.Reader, algorithm HashAlgorithm) (string, error) {
	switch algorithm {
	case AlgorithmBLAKE3:
		hasher := blake3.New()
		if _, err := io.Copy(hasher, reader); err != nil {
			return "", fmt.Errorf("failed to calculate BLAKE3 hash: %w", err)
		}
		return hex.EncodeToString(hasher.Sum(nil)), nil

	case AlgorithmSHA256:
		hasher := sha256.New()
		if _, err := io.Copy(hasher, reader); err != nil {
			return "", fmt.Errorf("failed to calculate SHA256 hash: %w", err)
		}
		return hex.EncodeToString(hasher.Sum(nil)), nil

	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}
}

// calculateQuickHash calculates a quick hash for large files
// Strategy: hash(first_chunk + last_chunk + file_size)
// This provides fast hashing for large video/audio files while maintaining
// reasonable collision resistance for the same file types
func calculateQuickHash(file *os.File, fileSize int64, algorithm HashAlgorithm) (string, error) {
	var hasher interface {
		io.Writer
		Sum([]byte) []byte
	}

	switch algorithm {
	case AlgorithmBLAKE3:
		hasher = blake3.New()
	case AlgorithmSHA256:
		hasher = sha256.New()
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	// Write file size as part of hash input
	fileSizeBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		fileSizeBytes[i] = byte(fileSize >> (i * 8))
	}
	hasher.Write(fileSizeBytes)

	// Read first chunk
	firstChunk := make([]byte, QuickHashChunkSize)
	n, err := file.ReadAt(firstChunk, 0)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read first chunk: %w", err)
	}
	hasher.Write(firstChunk[:n])

	// Read last chunk if file is large enough
	if fileSize > QuickHashChunkSize {
		lastChunkStart := fileSize - QuickHashChunkSize
		if lastChunkStart < QuickHashChunkSize {
			// Avoid overlap with first chunk
			lastChunkStart = QuickHashChunkSize
		}

		lastChunk := make([]byte, QuickHashChunkSize)
		n, err := file.ReadAt(lastChunk, lastChunkStart)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read last chunk: %w", err)
		}
		hasher.Write(lastChunk[:n])
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CalculateReaderHash calculates hash from an io.Reader
func CalculateReaderHash(reader io.Reader, algorithm HashAlgorithm) (string, error) {
	return calculateFullHash(reader, algorithm)
}

// ValidateHash checks if a hash string is valid for the given algorithm
func ValidateHash(hash string, algorithm HashAlgorithm) bool {
	switch algorithm {
	case AlgorithmBLAKE3:
		// BLAKE3 produces 256-bit (32 bytes) hash = 64 hex characters
		return len(hash) == 64 && isHexString(hash)
	case AlgorithmSHA256:
		// SHA256 produces 256-bit (32 bytes) hash = 64 hex characters
		return len(hash) == 64 && isHexString(hash)
	default:
		return false
	}
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// GetHashForAssetType returns the appropriate hash strategy for an asset type
// Photos: Full BLAKE3 (preferred from client) or SHA256
// Videos: Quick hash for large files, full for small
// Audio: Quick hash for large files, full for small
func GetHashForAssetType(filePath string, assetType string, clientHash string) (*HashResult, error) {
	// If client provided a valid hash, use it
	if clientHash != "" && (ValidateHash(clientHash, AlgorithmBLAKE3) || ValidateHash(clientHash, AlgorithmSHA256)) {
		stat, err := os.Stat(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file: %w", err)
		}

		// Determine algorithm from hash length and format
		algorithm := AlgorithmBLAKE3 // Assume BLAKE3 from client
		if !ValidateHash(clientHash, AlgorithmBLAKE3) {
			algorithm = AlgorithmSHA256
		}

		return &HashResult{
			Algorithm: algorithm,
			Hash:      clientHash,
			IsQuick:   false,
			FileSize:  stat.Size(),
		}, nil
	}

	// Fallback: calculate hash based on asset type
	switch assetType {
	case "PHOTO":
		// Photos: full BLAKE3 hash (preferred) or SHA256
		return CalculateFileHash(filePath, AlgorithmBLAKE3, false)

	case "VIDEO", "AUDIO":
		// Videos/Audio: use quick hash for large files
		return CalculateFileHash(filePath, AlgorithmBLAKE3, true)

	default:
		// Default: full BLAKE3 hash
		return CalculateFileHash(filePath, AlgorithmBLAKE3, false)
	}
}

// CompareHashes compares two hashes, handling different algorithms
// Returns true if hashes match (same algorithm and value)
func CompareHashes(hash1 string, algorithm1 HashAlgorithm, hash2 string, algorithm2 HashAlgorithm) bool {
	return algorithm1 == algorithm2 && hash1 == hash2
}
