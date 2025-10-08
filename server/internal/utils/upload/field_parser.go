package upload

import (
	"fmt"
	"strconv"
	"strings"
)

// FileFieldInfo represents parsed information from upload field names
type FileFieldInfo struct {
	Type        string // "single" or "chunk"
	SessionID   string // upload session ID
	ChunkIndex  int    // chunk index (0-based)
	TotalChunks int    // total number of chunks
	OriginalKey string // original field name
}

// ParseFileField parses upload field names to extract upload type and session information
func ParseFileField(fieldName string) (*FileFieldInfo, error) {
	parts := strings.Split(fieldName, "_")

	if len(parts) == 2 && parts[0] == "single" {
		// Single file format: single_{session_id}
		return &FileFieldInfo{
			Type:        "single",
			SessionID:   parts[1],
			ChunkIndex:  0,
			TotalChunks: 1,
			OriginalKey: fieldName,
		}, nil
	} else if len(parts) == 4 && parts[0] == "chunk" {
		// Chunk format: chunk_{session_id}_{chunk_index}_{total_chunks}
		chunkIndex, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid chunk index in field %s: %w", fieldName, err)
		}

		totalChunks, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid total chunks in field %s: %w", fieldName, err)
		}

		if chunkIndex < 0 {
			return nil, fmt.Errorf("chunk index cannot be negative in field %s", fieldName)
		}

		if totalChunks <= 0 {
			return nil, fmt.Errorf("total chunks must be positive in field %s", fieldName)
		}

		if chunkIndex >= totalChunks {
			return nil, fmt.Errorf("chunk index %d exceeds total chunks %d in field %s",
				chunkIndex, totalChunks, fieldName)
		}

		return &FileFieldInfo{
			Type:        "chunk",
			SessionID:   parts[1],
			ChunkIndex:  chunkIndex,
			TotalChunks: totalChunks,
			OriginalKey: fieldName,
		}, nil
	}

	return nil, fmt.Errorf("invalid file field format: %s. Expected 'single_{id}' or 'chunk_{id}_{index}_{total}'", fieldName)
}

// GenerateSingleFieldName generates a field name for single file upload
func GenerateSingleFieldName(sessionID string) string {
	return fmt.Sprintf("single_%s", sessionID)
}

// GenerateChunkFieldName generates a field name for chunk upload
func GenerateChunkFieldName(sessionID string, chunkIndex, totalChunks int) string {
	return fmt.Sprintf("chunk_%s_%d_%d", sessionID, chunkIndex, totalChunks)
}

// ValidateFieldName validates if a field name matches expected patterns
func ValidateFieldName(fieldName string) bool {
	_, err := ParseFileField(fieldName)
	return err == nil
}

// ExtractSessionID extracts session ID from any valid field name
func ExtractSessionID(fieldName string) (string, error) {
	info, err := ParseFileField(fieldName)
	if err != nil {
		return "", err
	}
	return info.SessionID, nil
}

// IsSingleUpload checks if field name represents a single file upload
func IsSingleUpload(fieldName string) bool {
	info, err := ParseFileField(fieldName)
	if err != nil {
		return false
	}
	return info.Type == "single"
}

// IsChunkUpload checks if field name represents a chunk upload
func IsChunkUpload(fieldName string) bool {
	info, err := ParseFileField(fieldName)
	if err != nil {
		return false
	}
	return info.Type == "chunk"
}

// GroupFieldsBySession groups field names by their session ID
func GroupFieldsBySession(fieldNames []string) (map[string][]string, error) {
	groups := make(map[string][]string)

	for _, fieldName := range fieldNames {
		sessionID, err := ExtractSessionID(fieldName)
		if err != nil {
			return nil, err
		}

		if groups[sessionID] == nil {
			groups[sessionID] = make([]string, 0)
		}
		groups[sessionID] = append(groups[sessionID], fieldName)
	}

	return groups, nil
}
