package handler

import (
	"database/sql"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func TestLoadUploadOperationStatusesReadsOwnedIngestReceipt(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	for _, statement := range []string{
		`CREATE TABLE catalog_operation_receipts (receipt_id TEXT PRIMARY KEY, kind TEXT NOT NULL, subject_id TEXT NOT NULL, state TEXT NOT NULL, terminal_error TEXT)`,
		`CREATE TABLE repository_staging_commits (commit_id TEXT PRIMARY KEY, original_filename TEXT NOT NULL, owner_id INTEGER NOT NULL)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	commitID := uuid.New()
	receiptID := uuid.New()
	if _, err := database.Exec(`INSERT INTO repository_staging_commits(commit_id, original_filename, owner_id) VALUES (?, ?, ?)`, commitID.String(), "owned.jpg", 7); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO catalog_operation_receipts(receipt_id, kind, subject_id, state) VALUES (?, 'ingest', ?, 'completed')`, receiptID.String(), commitID.String()); err != nil {
		t.Fatal(err)
	}

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("user_id", int32(7))
	statuses, err := (&AssetHandler{database: database}).loadUploadOperationStatuses(context, receiptID.String())
	if err != nil {
		t.Fatalf("load upload operation statuses: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("status count = %d, want 1", len(statuses))
	}
	status := statuses[0]
	if status.ReceiptID != receiptID.String() || status.FileName != "owned.jpg" || status.Status != "completed" || !status.Terminal || !status.Success {
		t.Fatalf("unexpected upload status: %+v", status)
	}
}
