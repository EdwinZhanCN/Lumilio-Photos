package app

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/catalog"
	"server/internal/catalog/dbtypes"
	"server/internal/catalog/repo"
)

// Foreground reads never touch SQLite's sole writer: while a writer
// transaction holds the one writer connection, setup status, the storage
// views, the native-task list, and the processing status still answer
// promptly, and so does opening an agent ref. A read bound to the writer pool, or a read that persists
// something on the way, waits behind the held writer and times out.
func TestForegroundReadsAnswerWhileTheWriterIsHeld(t *testing.T) {
	catalogDB := make(chan *catalog.DB, 1)
	server := startBlackboxServerWith(t, blackboxOptions{
		catalogOpened: func(database *catalog.DB) { catalogDB <- database },
	})
	database := <-catalogDB
	// One Asset, so opening the agent ref reads it through the authorized
	// library.
	const filename = "foreground-read.jpg"
	server.writeFile(server.primary, "photos/"+filename, blackboxJPEG(t, 40, "foreground-read"))
	server.scan(server.primary, 2*time.Minute)
	var listed []blackboxAsset
	if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
		t.Fatalf("scanned file never appeared in library browse: %+v", listed)
	}
	server.waitProcessingSettled()
	refPath := storedAgentRef(t, database, uuid.MustParse(listed[0].AssetID))
	readsWhileWriterHeld(t, server, database, []string{
		"/api/v1/setup/status",
		"/api/v1/storage/targets",
		"/api/v1/storage/view",
		"/api/v1/storage/native-tasks",
		"/api/v1/admin/processing",
		refPath,
	})
}

// storedAgentRef writes an agent ref over assetIDs for the administrator into the catalog,
// as an agent run in another process generation would have, and returns the
// path the Web uses to open it. Opening it reads the catalog row and builds
// facets through the authorized library.
func storedAgentRef(t *testing.T, database *catalog.DB, assetIDs ...uuid.UUID) string {
	t.Helper()
	ctx := context.Background()
	admin, err := database.Queries.GetUserByUsername(ctx, blackboxAdminUsername)
	if err != nil {
		t.Fatal(err)
	}
	const thread = "foreground-read"
	now := dbtypes.NewTimestamp(time.Now())
	if _, err := database.Queries.UpsertAgentThread(ctx, repo.UpsertAgentThreadParams{
		UserID: admin.UserID, ThreadID: thread, CheckpointKey: core.CheckpointKey(admin.UserID, thread),
		Mode: "free", ContextBindings: dbtypes.JSON("[]"), MentionBindings: dbtypes.JSON("[]"),
		PolicyVersion: core.CurrentAgentPolicyVersion, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	store := ref.NewPersistentStore(database.Queries, nil, ref.DefaultTTL, ref.DefaultMaxRefsPerScope, 1<<20, 1<<20)
	stored, refErr := store.Create(ctx, ref.Scope{UserID: admin.UserID, ThreadID: thread},
		ref.Plan{}, "", "foreground read probe", assetIDs, false)
	if refErr != nil {
		t.Fatal(refErr)
	}
	return "/api/v1/agent/refs/" + stored.ID + "?thread_id=" + thread
}

// Before setup finishes, setup status derives the phase on every request;
// it still never persists that phase through the writer.
func TestSetupStatusAnswersWhileTheWriterIsHeldBeforeSetupFinishes(t *testing.T) {
	catalogDB := make(chan *catalog.DB, 1)
	server := startBlackboxServerWith(t, blackboxOptions{
		skipPrimary:   true,
		catalogOpened: func(database *catalog.DB) { catalogDB <- database },
	})
	readsWhileWriterHeld(t, server, <-catalogDB, []string{"/api/v1/setup/status"})
}

func readsWhileWriterHeld(t *testing.T, server *blackboxServer, database *catalog.DB, reads []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	held, err := database.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Rollback()
	if _, err := held.ExecContext(ctx, `UPDATE system_state SET updated_at = updated_at WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	// The hold is real: a write through the catalog's own surface now waits.
	probeCtx, probeCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, probeErr := database.Queries.SetBootstrapPhase(probeCtx, "ready")
	probeCancel()
	if probeErr == nil {
		t.Fatal("a catalog write succeeded while the writer was held; the test holds nothing")
	}

	client := &http.Client{Timeout: 3 * time.Second}
	var wait sync.WaitGroup
	failures := make(chan string, len(reads))
	for _, path := range reads {
		wait.Add(1)
		go func() {
			defer wait.Done()
			request, err := http.NewRequest(http.MethodGet, server.baseURL+path, nil)
			if err != nil {
				failures <- path + ": " + err.Error()
				return
			}
			request.Header.Set("Authorization", "Bearer "+server.token)
			response, err := client.Do(request)
			if err != nil {
				failures <- path + ": " + err.Error()
				return
			}
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				failures <- path + ": status " + response.Status
			}
		}()
	}
	wait.Wait()
	close(failures)
	for failure := range failures {
		t.Errorf("foreground read waited for the held writer or failed: %s", failure)
	}
}
