package upload

import (
	"testing"
)

func TestParseFileField(t *testing.T) {
	tests := []struct {
		name        string
		fieldName   string
		wantType    string
		wantSession string
		wantIndex   int
		wantTotal   int
		wantErr     bool
	}{
		{
			name:        "single file format",
			fieldName:   "single_11111111-1111-4111-8111-111111111111",
			wantType:    "single",
			wantSession: "11111111-1111-4111-8111-111111111111",
			wantIndex:   0,
			wantTotal:   1,
			wantErr:     false,
		},
		{
			name:        "chunk file format",
			fieldName:   "chunk_22222222-2222-4222-8222-222222222222_0_10",
			wantType:    "chunk",
			wantSession: "22222222-2222-4222-8222-222222222222",
			wantIndex:   0,
			wantTotal:   10,
			wantErr:     false,
		},
		{
			name:        "middle chunk",
			fieldName:   "chunk_33333333-3333-4333-8333-333333333333_5_20",
			wantType:    "chunk",
			wantSession: "33333333-3333-4333-8333-333333333333",
			wantIndex:   5,
			wantTotal:   20,
			wantErr:     false,
		},
		{
			name:      "invalid format",
			fieldName: "invalid_format",
			wantErr:   true,
		},
		{
			name:      "session id is not a UUID",
			fieldName: "single_abc123",
			wantErr:   true,
		},
		{
			name:      "invalid chunk index",
			fieldName: "chunk_22222222-2222-4222-8222-222222222222_abc_10",
			wantErr:   true,
		},
		{
			name:      "chunk index out of bounds",
			fieldName: "chunk_22222222-2222-4222-8222-222222222222_15_10",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFileField(tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFileField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.Type != tt.wantType {
				t.Errorf("ParseFileField() Type = %v, want %v", got.Type, tt.wantType)
			}
			if got.SessionID != tt.wantSession {
				t.Errorf("ParseFileField() SessionID = %v, want %v", got.SessionID, tt.wantSession)
			}
			if got.ChunkIndex != tt.wantIndex {
				t.Errorf("ParseFileField() ChunkIndex = %v, want %v", got.ChunkIndex, tt.wantIndex)
			}
			if got.TotalChunks != tt.wantTotal {
				t.Errorf("ParseFileField() TotalChunks = %v, want %v", got.TotalChunks, tt.wantTotal)
			}
		})
	}
}

func TestGenerateFieldNames(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		chunkIndex  int
		totalChunks int
		wantSingle  string
		wantChunk   string
	}{
		{
			name:        "basic session",
			sessionID:   "test123",
			chunkIndex:  0,
			totalChunks: 5,
			wantSingle:  "single_test123",
			wantChunk:   "chunk_test123_0_5",
		},
		{
			name:        "middle chunk",
			sessionID:   "session456",
			chunkIndex:  3,
			totalChunks: 10,
			wantSingle:  "single_session456",
			wantChunk:   "chunk_session456_3_10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test single file generation
			gotSingle := GenerateSingleFieldName(tt.sessionID)
			if gotSingle != tt.wantSingle {
				t.Errorf("GenerateSingleFieldName() = %v, want %v", gotSingle, tt.wantSingle)
			}

			// Test chunk file generation
			gotChunk := GenerateChunkFieldName(tt.sessionID, tt.chunkIndex, tt.totalChunks)
			if gotChunk != tt.wantChunk {
				t.Errorf("GenerateChunkFieldName() = %v, want %v", gotChunk, tt.wantChunk)
			}
		})
	}
}

func TestValidateFieldName(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		want      bool
	}{
		{"valid single", "single_11111111-1111-4111-8111-111111111111", true},
		{"valid chunk", "chunk_22222222-2222-4222-8222-222222222222_0_10", true},
		{"invalid format", "invalid", false},
		{"empty string", "", false},
		{"missing parts", "single", false},
		{"chunk missing parts", "chunk_session", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateFieldName(tt.fieldName); got != tt.want {
				t.Errorf("ValidateFieldName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSingleUpload(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		want      bool
	}{
		{"single file", "single_11111111-1111-4111-8111-111111111111", true},
		{"chunk file", "chunk_22222222-2222-4222-8222-222222222222_0_10", false},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSingleUpload(tt.fieldName); got != tt.want {
				t.Errorf("IsSingleUpload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsChunkUpload(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		want      bool
	}{
		{"chunk file", "chunk_22222222-2222-4222-8222-222222222222_0_10", true},
		{"single file", "single_11111111-1111-4111-8111-111111111111", false},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsChunkUpload(tt.fieldName); got != tt.want {
				t.Errorf("IsChunkUpload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		want      string
		wantErr   bool
	}{
		{"single file", "single_11111111-1111-4111-8111-111111111111", "11111111-1111-4111-8111-111111111111", false},
		{"chunk file", "chunk_22222222-2222-4222-8222-222222222222_0_10", "22222222-2222-4222-8222-222222222222", false},
		{"invalid", "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractSessionID(tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractSessionID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractSessionID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupFieldsBySession(t *testing.T) {
	fieldNames := []string{
		"single_11111111-1111-4111-8111-111111111111",
		"chunk_22222222-2222-4222-8222-222222222222_0_5",
		"chunk_22222222-2222-4222-8222-222222222222_1_5",
		"single_33333333-3333-4333-8333-333333333333",
		"chunk_22222222-2222-4222-8222-222222222222_2_5",
	}

	groups, err := GroupFieldsBySession(fieldNames)
	if err != nil {
		t.Fatalf("GroupFieldsBySession() unexpected error: %v", err)
	}

	// Check session1
	if len(groups["11111111-1111-4111-8111-111111111111"]) != 1 {
		t.Errorf("Expected 1 field for session1, got %d", len(groups["11111111-1111-4111-8111-111111111111"]))
	}

	// Check session2
	if len(groups["22222222-2222-4222-8222-222222222222"]) != 3 {
		t.Errorf("Expected 3 fields for session2, got %d", len(groups["22222222-2222-4222-8222-222222222222"]))
	}

	// Check session3
	if len(groups["33333333-3333-4333-8333-333333333333"]) != 1 {
		t.Errorf("Expected 1 field for session3, got %d", len(groups["33333333-3333-4333-8333-333333333333"]))
	}

	// Test with invalid field name
	invalidFields := []string{"single_11111111-1111-4111-8111-111111111111", "invalid_field"}
	_, err = GroupFieldsBySession(invalidFields)
	if err == nil {
		t.Error("Expected error for invalid field name, got nil")
	}
}
