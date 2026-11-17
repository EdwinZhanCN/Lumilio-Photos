package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/ingest/upload"
	"server/internal/storage/layout"

	"github.com/google/uuid"
)

// TestCompletedChunkedUploadLeavesNoSession: a chunked upload that completed
// leaves no session behind, neither the in-memory session the progress
// endpoint reports nor any file in the private classes that open and adopt
// treat as an uncommitted upload, nor the session's directory.
func TestCompletedChunkedUploadLeavesNoSession(t *testing.T) {
	s := startBlackboxServer(t)
	contents := blackboxJPEG(t, 120, "chunked upload session")
	chunks := [][]byte{contents[:len(contents)/2], contents[len(contents)/2:]}
	sessionID := uuid.NewString()

	var created struct {
		SessionID string `json:"session_id"`
	}
	s.mustJSON(http.MethodPost, "/api/v1/assets/batch/sessions", map[string]any{
		"session_id": sessionID, "filename": "chunked.jpg", "total_size": len(contents),
		"total_chunks": len(chunks), "content_type": "image/jpeg", "repository_id": s.primary.ID,
	}, &created)
	if created.SessionID != sessionID {
		t.Fatalf("created session %q, want %q", created.SessionID, sessionID)
	}

	var receiptID string
	for index, chunk := range chunks {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("repository_id", s.primary.ID); err != nil {
			t.Fatal(err)
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename="chunked.jpg"`, upload.GenerateChunkFieldName(sessionID, index, len(chunks))))
		header.Set("Content-Type", "image/jpeg")
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(chunk); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		status, payload := s.do(http.MethodPost, "/api/v1/assets/batch", &body, writer.FormDataContentType())
		if status != http.StatusOK {
			t.Fatalf("chunk %d: %d %s", index, status, payload)
		}
		var response struct {
			Results []struct {
				Success   bool    `json:"success"`
				ReceiptID *string `json:"receipt_id"`
			} `json:"results"`
		}
		if err := json.Unmarshal(payload, &response); err != nil || len(response.Results) != 1 || !response.Results[0].Success {
			t.Fatalf("chunk %d: %s (%v)", index, payload, err)
		}
		if response.Results[0].ReceiptID != nil {
			receiptID = *response.Results[0].ReceiptID
		}
	}
	if receiptID == "" {
		t.Fatal("the last chunk returned no ingest receipt")
	}
	completed := waitFor(2*time.Minute, func() bool {
		var response struct {
			Operations []struct {
				Terminal bool `json:"terminal"`
				Success  bool `json:"success"`
			} `json:"operations"`
		}
		s.mustJSON(http.MethodGet, "/api/v1/assets/batch/operations?receipt_ids="+receiptID, nil, &response)
		return len(response.Operations) == 1 && response.Operations[0].Terminal && response.Operations[0].Success
	})
	if !completed {
		t.Fatalf("receipt %s did not complete", receiptID)
	}

	var progress struct {
		Sessions []struct {
			SessionID string `json:"session_id"`
			Status    string `json:"status"`
		} `json:"sessions"`
	}
	s.mustJSON(http.MethodGet, "/api/v1/assets/batch/progress?session_ids="+sessionID, nil, &progress)
	if len(progress.Sessions) != 0 {
		t.Errorf("the completed session is still held in memory: %+v", progress.Sessions)
	}

	// Open and adopt move whatever work/ holds under a recovery receipt.
	work, err := layout.ClassDir(layout.Work)
	if err != nil {
		t.Fatal(err)
	}
	local, err := work.Local()
	if err != nil {
		t.Fatal(err)
	}
	var leftovers []string
	_ = filepath.WalkDir(filepath.Join(s.primary.Path, local), func(name string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			relative, _ := filepath.Rel(s.primary.Path, name)
			leftovers = append(leftovers, filepath.ToSlash(relative))
		}
		return nil
	})
	session, err := layout.UploadSession(uuid.MustParse(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if local, err := session.Local(); err != nil {
		t.Fatal(err)
	} else if _, err := os.Lstat(filepath.Join(s.primary.Path, local)); !os.IsNotExist(err) {
		t.Errorf("the completed session's directory remains: %v", err)
	}
	if len(leftovers) != 0 {
		t.Errorf("the completed session left private files that open and adopt would recover as an uncommitted upload: %v", leftovers)
	}
}
