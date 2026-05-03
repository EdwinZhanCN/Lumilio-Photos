package jobs

import (
	"encoding/json"
	"testing"
)

func TestProcessArgsDecodeLegacyImageDataWithoutPersistingBytes(t *testing.T) {
	tests := map[string]any{
		"clip":    &ProcessClipArgs{},
		"ocr":     &ProcessOcrArgs{},
		"caption": &ProcessCaptionArgs{},
		"face":    &ProcessFaceArgs{},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			err := json.Unmarshal([]byte(`{
				"assetId": "11111111-1111-1111-1111-111111111111",
				"imageData": "aW1hZ2U=",
				"preprocessVersion": "ml-image-v1"
			}`), args)
			if err != nil {
				t.Fatalf("decode args: %v", err)
			}

			encoded, err := json.Marshal(args)
			if err != nil {
				t.Fatalf("marshal args: %v", err)
			}
			if !json.Valid(encoded) {
				t.Fatalf("expected valid json, got %s", encoded)
			}
			if containsJSONField(encoded, "imageData") {
				t.Fatalf("expected marshaled args to omit legacy imageData: %s", encoded)
			}
		})
	}
}

func containsJSONField(data []byte, field string) bool {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return false
	}
	_, ok := decoded[field]
	return ok
}
