package file

import (
	"server/internal/db/dbtypes"
	"strings"
	"testing"
)

func TestValidator_ValidateFile(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		filename    string
		contentType string
		wantValid   bool
		wantType    dbtypes.AssetType
		wantIsRAW   bool
		wantError   string
	}{
		// Valid photo formats
		{
			name:        "Valid JPEG",
			filename:    "photo.jpg",
			contentType: "image/jpeg",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   false,
		},
		{
			name:        "Valid PNG",
			filename:    "image.png",
			contentType: "image/png",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   false,
		},
		{
			name:        "Valid HEIC",
			filename:    "photo.heic",
			contentType: "image/heic",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   false,
		},
		// Valid RAW formats
		{
			name:        "Valid Canon CR2",
			filename:    "IMG_1234.CR2",
			contentType: "image/x-canon-cr2",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   true,
		},
		{
			name:        "Valid Nikon NEF",
			filename:    "DSC_5678.nef",
			contentType: "image/x-nikon-nef",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   true,
		},
		{
			name:        "Valid Sony ARW",
			filename:    "photo.arw",
			contentType: "image/x-sony-arw",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   true,
		},
		{
			name:        "Valid DNG",
			filename:    "photo.dng",
			contentType: "image/x-adobe-dng",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   true,
		},
		// Valid video formats
		{
			name:        "Valid MP4",
			filename:    "video.mp4",
			contentType: "video/mp4",
			wantValid:   true,
			wantType:    dbtypes.AssetTypeVideo,
			wantIsRAW:   false,
		},
		{
			name:        "Valid MOV",
			filename:    "video.mov",
			contentType: "video/quicktime",
			wantValid:   true,
			wantType:    dbtypes.AssetTypeVideo,
			wantIsRAW:   false,
		},
		{
			name:        "Valid MKV",
			filename:    "video.mkv",
			contentType: "video/x-matroska",
			wantValid:   true,
			wantType:    dbtypes.AssetTypeVideo,
			wantIsRAW:   false,
		},
		// Valid audio formats
		{
			name:        "Valid MP3",
			filename:    "song.mp3",
			contentType: "audio/mpeg",
			wantValid:   true,
			wantType:    dbtypes.AssetTypeAudio,
			wantIsRAW:   false,
		},
		{
			name:        "Valid FLAC",
			filename:    "song.flac",
			contentType: "audio/flac",
			wantValid:   true,
			wantType:    dbtypes.AssetTypeAudio,
			wantIsRAW:   false,
		},
		{
			name:        "Valid M4A",
			filename:    "song.m4a",
			contentType: "audio/mp4",
			wantValid:   true,
			wantType:    dbtypes.AssetTypeAudio,
			wantIsRAW:   false,
		},
		// Invalid cases
		{
			name:        "No extension",
			filename:    "file",
			contentType: "image/jpeg",
			wantValid:   false,
			wantError:   "file has no extension",
		},
		{
			name:        "Unsupported extension",
			filename:    "document.pdf",
			contentType: "application/pdf",
			wantValid:   false,
			wantError:   "unsupported file extension: .pdf",
		},
		{
			name:        "Mismatched MIME type",
			filename:    "photo.jpg",
			contentType: "video/mp4",
			wantValid:   false,
			wantError:   "MIME type 'video/mp4' does not match file extension '.jpg'",
		},
		// Case insensitive
		{
			name:        "Uppercase extension",
			filename:    "PHOTO.JPG",
			contentType: "image/jpeg",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   false,
		},
		// Empty MIME type (should still work based on extension)
		{
			name:        "Empty MIME type with valid extension",
			filename:    "photo.jpg",
			contentType: "",
			wantValid:   true,
			wantType:    dbtypes.AssetTypePhoto,
			wantIsRAW:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateFile(tt.filename, tt.contentType)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateFile() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.Valid {
				if result.AssetType != tt.wantType {
					t.Errorf("ValidateFile() AssetType = %v, want %v", result.AssetType, tt.wantType)
				}
				if result.IsRAW != tt.wantIsRAW {
					t.Errorf("ValidateFile() IsRAW = %v, want %v", result.IsRAW, tt.wantIsRAW)
				}
			} else {
				if tt.wantError != "" && result.ErrorReason != tt.wantError {
					t.Errorf("ValidateFile() ErrorReason = %v, want %v", result.ErrorReason, tt.wantError)
				}
			}
		})
	}
}

func TestValidator_IsSupported(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"JPEG", "photo.jpg", true},
		{"PNG", "image.png", true},
		{"HEIC", "photo.heic", true},
		{"CR2", "IMG_1234.CR2", true},
		{"NEF", "DSC_5678.nef", true},
		{"MP4", "video.mp4", true},
		{"MOV", "video.mov", true},
		{"MP3", "song.mp3", true},
		{"FLAC", "song.flac", true},
		{"PDF", "document.pdf", false},
		{"TXT", "readme.txt", false},
		{"No extension", "file", false},
		{"Uppercase", "PHOTO.JPG", true},
		{"Mixed case", "Photo.JpEg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.IsSupported(tt.filename)
			if got != tt.want {
				t.Errorf("IsSupported(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestValidator_IsSupportedExtension(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name string
		ext  string
		want bool
	}{
		{"With dot", ".jpg", true},
		{"Without dot", "jpg", true},
		{"Uppercase with dot", ".JPG", true},
		{"Uppercase without dot", "JPG", true},
		{"RAW format", ".cr2", true},
		{"Video format", ".mp4", true},
		{"Audio format", ".mp3", true},
		{"Unsupported", ".pdf", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.IsSupportedExtension(tt.ext)
			if got != tt.want {
				t.Errorf("IsSupportedExtension(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestValidator_GetAssetTypeByExtension(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		ext     string
		wantOk  bool
		wantTyp dbtypes.AssetType
	}{
		{".jpg", ".jpg", true, dbtypes.AssetTypePhoto},
		{".png", ".png", true, dbtypes.AssetTypePhoto},
		{".cr2", ".cr2", true, dbtypes.AssetTypePhoto},
		{".nef", ".nef", true, dbtypes.AssetTypePhoto},
		{".mp4", ".mp4", true, dbtypes.AssetTypeVideo},
		{".mov", ".mov", true, dbtypes.AssetTypeVideo},
		{".mp3", ".mp3", true, dbtypes.AssetTypeAudio},
		{".flac", ".flac", true, dbtypes.AssetTypeAudio},
		{".pdf", ".pdf", false, ""},
		{"empty", "", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTyp, gotOk := validator.GetAssetTypeByExtension(tt.ext)
			if gotOk != tt.wantOk {
				t.Errorf("GetAssetTypeByExtension(%q) ok = %v, want %v", tt.ext, gotOk, tt.wantOk)
			}
			if gotOk && gotTyp != tt.wantTyp {
				t.Errorf("GetAssetTypeByExtension(%q) type = %v, want %v", tt.ext, gotTyp, tt.wantTyp)
			}
		})
	}
}

func TestValidator_GetAssetTypeByMimeType(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name     string
		mimeType string
		wantOk   bool
		wantTyp  dbtypes.AssetType
	}{
		{"JPEG", "image/jpeg", true, dbtypes.AssetTypePhoto},
		{"PNG", "image/png", true, dbtypes.AssetTypePhoto},
		{"Generic image", "image/something", true, dbtypes.AssetTypePhoto},
		{"MP4 video", "video/mp4", true, dbtypes.AssetTypeVideo},
		{"Generic video", "video/something", true, dbtypes.AssetTypeVideo},
		{"MP3 audio", "audio/mpeg", true, dbtypes.AssetTypeAudio},
		{"Generic audio", "audio/something", true, dbtypes.AssetTypeAudio},
		{"PDF", "application/pdf", false, ""},
		{"Empty", "", false, ""},
		{"Whitespace", "  image/jpeg  ", true, dbtypes.AssetTypePhoto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTyp, gotOk := validator.GetAssetTypeByMimeType(tt.mimeType)
			if gotOk != tt.wantOk {
				t.Errorf("GetAssetTypeByMimeType(%q) ok = %v, want %v", tt.mimeType, gotOk, tt.wantOk)
			}
			if gotOk && gotTyp != tt.wantTyp {
				t.Errorf("GetAssetTypeByMimeType(%q) type = %v, want %v", tt.mimeType, gotTyp, tt.wantTyp)
			}
		})
	}
}

func TestValidator_DetermineAssetType(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		filename    string
		contentType string
		want        dbtypes.AssetType
	}{
		{"Both provided", "photo.jpg", "image/jpeg", dbtypes.AssetTypePhoto},
		{"Only filename", "photo.jpg", "", dbtypes.AssetTypePhoto},
		{"Only content type", "", "image/jpeg", dbtypes.AssetTypePhoto},
		{"Mismatched (prefers extension)", "photo.jpg", "video/mp4", dbtypes.AssetTypePhoto},
		{"Video file", "movie.mp4", "video/mp4", dbtypes.AssetTypeVideo},
		{"Audio file", "song.mp3", "audio/mpeg", dbtypes.AssetTypeAudio},
		{"RAW file", "IMG_1234.CR2", "image/x-canon-cr2", dbtypes.AssetTypePhoto},
		{"Neither provided (fallback)", "", "", dbtypes.AssetTypePhoto},
		{"Unsupported extension (uses mime)", "file.unknown", "video/mp4", dbtypes.AssetTypeVideo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.DetermineAssetType(tt.filename, tt.contentType)
			if got != tt.want {
				t.Errorf("DetermineAssetType(%q, %q) = %v, want %v", tt.filename, tt.contentType, got, tt.want)
			}
		})
	}
}

func TestValidator_IsValidMimeType(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		mimeType  string
		assetType dbtypes.AssetType
		want      bool
	}{
		{"Valid photo MIME", "image/jpeg", dbtypes.AssetTypePhoto, true},
		{"Valid video MIME", "video/mp4", dbtypes.AssetTypeVideo, true},
		{"Valid audio MIME", "audio/mpeg", dbtypes.AssetTypeAudio, true},
		{"Invalid photo MIME", "video/mp4", dbtypes.AssetTypePhoto, false},
		{"Invalid video MIME", "image/jpeg", dbtypes.AssetTypeVideo, false},
		{"Invalid audio MIME", "image/jpeg", dbtypes.AssetTypeAudio, false},
		{"Generic image MIME", "image/unknown", dbtypes.AssetTypePhoto, true},
		{"Generic video MIME", "video/unknown", dbtypes.AssetTypeVideo, true},
		{"Generic audio MIME", "audio/unknown", dbtypes.AssetTypeAudio, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.IsValidMimeType(tt.mimeType, tt.assetType)
			if got != tt.want {
				t.Errorf("IsValidMimeType(%q, %v) = %v, want %v", tt.mimeType, tt.assetType, got, tt.want)
			}
		})
	}
}

func TestValidator_IsRAWFile(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"CR2", "IMG_1234.CR2", true},
		{"CR3", "IMG_5678.cr3", true},
		{"NEF", "DSC_9012.nef", true},
		{"ARW", "photo.arw", true},
		{"DNG", "photo.dng", true},
		{"ORF", "photo.orf", true},
		{"RW2", "photo.rw2", true},
		{"PEF", "photo.pef", true},
		{"RAF", "photo.raf", true},
		{"Not RAW JPG", "photo.jpg", false},
		{"Not RAW PNG", "image.png", false},
		{"Not RAW MP4", "video.mp4", false},
		{"Uppercase", "PHOTO.CR2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.IsRAWFile(tt.filename)
			if got != tt.want {
				t.Errorf("IsRAWFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestValidator_GetMimeTypeFromExtension(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		ext         string
		wantContain string // Check if result contains this string
	}{
		{"JPEG", ".jpg", "image/jpeg"},
		{"PNG", ".png", "image/png"},
		{"CR2", ".cr2", "image/x-canon-cr2"},
		{"CR3", ".cr3", "image/x-canon-cr3"},
		{"NEF", ".nef", "image/x-nikon-nef"},
		{"MP4", ".mp4", "video/mp4"},
		{"M4A", ".m4a", "audio/"},
		{"WebM", ".webm", "video/webm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.GetMimeTypeFromExtension(tt.ext)
			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("GetMimeTypeFromExtension(%q) = %v, want to contain %v", tt.ext, got, tt.wantContain)
			}
		})
	}
}

func TestValidator_GetSupportedExtensions(t *testing.T) {
	validator := NewValidator()
	extensions := validator.GetSupportedExtensions()

	if len(extensions) == 0 {
		t.Error("GetSupportedExtensions() returned empty slice")
	}

	// Check for some expected extensions
	expectedExts := []string{".jpg", ".png", ".mp4", ".mp3", ".cr2", ".nef"}
	for _, ext := range expectedExts {
		found := false
		for _, e := range extensions {
			if e == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetSupportedExtensions() missing expected extension: %s", ext)
		}
	}
}

func TestValidator_GetSupportedExtensionsByType(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name             string
		assetType        dbtypes.AssetType
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:             "Photo extensions",
			assetType:        dbtypes.AssetTypePhoto,
			shouldContain:    []string{".jpg", ".png", ".cr2", ".nef"},
			shouldNotContain: []string{".mp4", ".mp3"},
		},
		{
			name:             "Video extensions",
			assetType:        dbtypes.AssetTypeVideo,
			shouldContain:    []string{".mp4", ".mov", ".avi"},
			shouldNotContain: []string{".jpg", ".mp3"},
		},
		{
			name:             "Audio extensions",
			assetType:        dbtypes.AssetTypeAudio,
			shouldContain:    []string{".mp3", ".flac", ".wav"},
			shouldNotContain: []string{".jpg", ".mp4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensions := validator.GetSupportedExtensionsByType(tt.assetType)

			for _, ext := range tt.shouldContain {
				found := false
				for _, e := range extensions {
					if e == ext {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetSupportedExtensionsByType(%v) missing expected extension: %s", tt.assetType, ext)
				}
			}

			for _, ext := range tt.shouldNotContain {
				for _, e := range extensions {
					if e == ext {
						t.Errorf("GetSupportedExtensionsByType(%v) should not contain: %s", tt.assetType, ext)
					}
				}
			}
		})
	}
}

func TestValidator_FormatValidationError(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name   string
		result *ValidationResult
		want   string
	}{
		{
			name: "Valid result",
			result: &ValidationResult{
				Valid: true,
			},
			want: "",
		},
		{
			name: "Invalid with reason",
			result: &ValidationResult{
				Valid:       false,
				ErrorReason: "unsupported file type",
			},
			want: "unsupported file type",
		},
		{
			name: "Invalid without reason",
			result: &ValidationResult{
				Valid: false,
			},
			want: "file validation failed for unknown reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.FormatValidationError(tt.result)
			if got != tt.want {
				t.Errorf("FormatValidationError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Backward compatibility tests
func TestDetermineAssetType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        dbtypes.AssetType
	}{
		{"Image JPEG", "image/jpeg", dbtypes.AssetTypePhoto},
		{"Video MP4", "video/mp4", dbtypes.AssetTypeVideo},
		{"Audio MP3", "audio/mpeg", dbtypes.AssetTypeAudio},
		{"Unknown", "application/pdf", dbtypes.AssetTypePhoto}, // Fallback
		{"Empty", "", dbtypes.AssetTypePhoto},                  // Fallback
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineAssetType(tt.contentType)
			if got != tt.want {
				t.Errorf("DetermineAssetType(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}

func TestIsRAWFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"CR2", "photo.cr2", true},
		{"NEF", "photo.nef", true},
		{"JPG", "photo.jpg", false},
		{"MP4", "video.mp4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRAWFile(tt.filename)
			if got != tt.want {
				t.Errorf("IsRAWFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}
