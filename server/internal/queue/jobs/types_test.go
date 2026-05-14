package jobs

import (
	"encoding/json"
	"testing"
)

func TestProcessArgsDecodeLegacyImageDataWithoutPersistingBytes(t *testing.T) {
	tests := map[string]any{
		"clip":    &ProcessClipArgs{},
		"bioclip": &ProcessBioClipArgs{},
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

func TestScanRepositoryArgsKindAndInsertOpts(t *testing.T) {
	args := ScanRepositoryArgs{
		RepositoryID: "11111111-1111-1111-1111-111111111111",
		Mode:         RepositoryScanModeManual,
		Force:        true,
	}

	if args.Kind() != "scan_repository" {
		t.Fatalf("unexpected kind: %s", args.Kind())
	}
	opts := args.InsertOpts()
	if !opts.UniqueOpts.ByArgs {
		t.Fatalf("expected scan repository jobs to be unique by args")
	}
	if opts.UniqueOpts.ByPeriod == 0 {
		t.Fatalf("expected scan repository jobs to use uniqueness by period")
	}
}

func TestProcessPHashArgsInsertOpts(t *testing.T) {
	args := ProcessPHashArgs{}

	if args.Kind() != "process_phash" {
		t.Fatalf("unexpected kind: %s", args.Kind())
	}

	opts := args.InsertOpts()
	if !opts.UniqueOpts.ByArgs {
		t.Fatalf("expected process pHash jobs to be unique by args")
	}
	if opts.UniqueOpts.ByPeriod == 0 {
		t.Fatalf("expected process pHash jobs to use uniqueness by period")
	}
}

func TestDiscoverAssetArgsInsertOptsAreUniqueByPath(t *testing.T) {
	args := DiscoverAssetArgs{
		RepositoryID: "11111111-1111-1111-1111-111111111111",
		RelativePath: "album/photo.jpg",
		Operation:    DiscoverOperationUpsert,
		FileName:     "photo.jpg",
	}

	if args.Kind() != "discover_asset" {
		t.Fatalf("unexpected kind: %s", args.Kind())
	}
	opts := args.InsertOpts()
	if !opts.UniqueOpts.ByArgs {
		t.Fatalf("expected discover asset jobs to be unique by args")
	}
	if opts.UniqueOpts.ByPeriod == 0 {
		t.Fatalf("expected discover asset jobs to use uniqueness by period")
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
