package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"
)

func TestDownloadStagesOnlyVerifiedArtifact(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("signed desktop update")
	manifest, err := SignForTest(Manifest{
		SchemaVersion: ManifestSchemaVersion,
		Channel:       "stable",
		Version:       "0.2.0",
		URL:           "https://updates.example.invalid/desktop.zip",
		SHA256:        sha256Hex(artifact),
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := jsonBytes(manifest)
	if err != nil {
		t.Fatal(err)
	}
	operations := operation.New()
	store := state.NewWithInstanceID("update-test")
	t.Cleanup(func() {
		operations.Close()
		store.Close()
	})
	controller := NewController(Options{
		Store: store, Operations: operations, PublicKey: publicKey,
		StagingDir: filepath.Join(t.TempDir(), "staging"),
		Fetch:      func(context.Context, string) ([]byte, []byte, error) { return manifestBytes, artifact, nil },
	})
	check, err := controller.Check("check-1", 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	waitUpdateOperation(t, operations, check.OperationID)
	if snapshot := controller.Snapshot(); snapshot.Phase != "available" || snapshot.CanApply {
		t.Fatalf("check snapshot = %#v, want available and not applicable", snapshot)
	}
	download, err := controller.Download("download-1", controller.Snapshot().Version)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	waitUpdateOperation(t, operations, download.OperationID)
	if snapshot := controller.Snapshot(); snapshot.Phase != "ready" || snapshot.CanApply {
		t.Fatalf("download snapshot = %#v, want ready and gated", snapshot)
	}
	if _, err := os.Stat(filepath.Join(controller.stagingDir, "current.json")); err != nil {
		t.Fatalf("staged pointer: %v", err)
	}
	controller.SetApply(func(string, uint64) (dto.OperationReceipt, error) {
		return dto.OperationReceipt{OperationID: "shutdown-1"}, nil
	})
	if !controller.Snapshot().CanApply {
		t.Fatal("verified staged update did not become applicable after apply handoff was configured")
	}
	receipt, err := controller.RestartAndApply("apply-1", controller.Snapshot().Version)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if receipt.OperationID == "" {
		t.Fatal("apply returned an empty receipt")
	}
	waitUpdateOperation(t, operations, receipt.OperationID)
}

func waitUpdateOperation(t *testing.T, registry *operation.Registry, operationID string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		item, ok := registry.Get(operationID)
		if ok && (item.State == string(operation.Succeeded) || item.State == string(operation.Failed)) {
			if item.State != string(operation.Succeeded) {
				t.Fatalf("operation %s failed: %#v", operationID, item)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("operation %s did not finish", operationID)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func jsonBytes(value any) ([]byte, error) { return json.Marshal(value) }
