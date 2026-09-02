package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"server/config"
	"server/internal/agent/ref"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/settings"

	"github.com/cloudwego/eino/adk"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type llmConfigProviderFunc func(context.Context) (settings.LLM, error)

func (f llmConfigProviderFunc) GetLLMConfig(ctx context.Context) (settings.LLM, error) {
	return f(ctx)
}

type awaitingAgentFixture struct {
	catalog  *db.DB
	service  *agentService
	user     repo.User
	thread   repo.AgentThread
	oldRunID uuid.UUID
}

func TestEnsureThreadPersistsMissingBindingsAsJSONArrays(t *testing.T) {
	ctx := context.Background()
	catalogDir := t.TempDir()
	require.NoError(t, os.Chmod(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "agent-service.sqlite3"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, catalog.Close(context.Background()))
	})
	require.NoError(t, catalog.Migrate(ctx))

	user, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username:           "agent-service-test",
		Password:           "unused",
		DisplayName:        "Agent Service Test",
		Role:               "user",
		WebauthnUserHandle: []byte("agent-service-test-handle"),
	})
	require.NoError(t, err)

	service := &agentService{queries: catalog.Queries}
	_, err = service.EnsureThread(ctx, user.UserID, "missing-bindings", "free", ThreadBindings{
		Context:  json.RawMessage("null"),
		Mentions: json.RawMessage("null"),
	})
	require.NoError(t, err)

	thread, err := catalog.Queries.GetAgentThread(ctx, repo.GetAgentThreadParams{
		UserID: user.UserID, ThreadID: "missing-bindings",
	})
	require.NoError(t, err)
	require.JSONEq(t, "[]", string(thread.ContextBindings))
	require.JSONEq(t, "[]", string(thread.MentionBindings))
}

func TestPreparedResumeTransitionActivatesOnlyAfterAwaitingRunCompletes(t *testing.T) {
	ctx := context.Background()
	fixture := newAwaitingAgentFixture(t, nil)
	preparedRunID, err := fixture.service.createPreparedRunRecord(ctx, fixture.thread)
	require.NoError(t, err)

	prepared, err := fixture.catalog.Queries.GetAgentRun(ctx, repo.GetAgentRunParams{
		RunID: preparedRunID, UserID: fixture.user.UserID, ThreadID: fixture.thread.ThreadID,
	})
	require.NoError(t, err)
	require.Equal(t, "running", prepared.Status)
	require.Equal(t, "prepared_resume", prepared.ActivationState)

	require.NoError(t, fixture.service.transitionAwaitingRun(
		ctx, fixture.thread, fixture.oldRunID, preparedRunID,
	))

	oldRun := getAgentRun(t, fixture, fixture.oldRunID)
	require.Equal(t, "completed", oldRun.Status)
	require.Equal(t, "terminal", oldRun.ActivationState)
	newRun := getAgentRun(t, fixture, preparedRunID)
	require.Equal(t, "running", newRun.Status)
	require.Equal(t, "active", newRun.ActivationState)
	thread, err := fixture.catalog.Queries.GetAgentThread(ctx, repo.GetAgentThreadParams{
		UserID: fixture.user.UserID, ThreadID: fixture.thread.ThreadID,
	})
	require.NoError(t, err)
	require.Equal(t, preparedRunID, thread.ActiveRunID.UUID)
}

func TestPreparedResumeTransitionFailureKeepsAwaitingRunRetryable(t *testing.T) {
	ctx := context.Background()
	fixture := newAwaitingAgentFixture(t, nil)
	preparedRunID, err := fixture.service.createPreparedRunRecord(ctx, fixture.thread)
	require.NoError(t, err)
	require.NoError(t, fixture.catalog.Queries.FinishAgentRun(ctx, repo.FinishAgentRunParams{
		Status: "failed", UpdatedAt: dbtypes.NewTimestamp(time.Now()),
		RunID: preparedRunID, UserID: fixture.user.UserID, ThreadID: fixture.thread.ThreadID,
	}))

	err = fixture.service.transitionAwaitingRun(ctx, fixture.thread, fixture.oldRunID, preparedRunID)
	require.Error(t, err)
	assertAwaitingRunRetryable(t, fixture)
	prepared := getAgentRun(t, fixture, preparedRunID)
	require.Equal(t, "failed", prepared.Status)
	require.Equal(t, "terminal", prepared.ActivationState)
}

func TestResumeCheckpointFailureLeavesAwaitingRunRetryable(t *testing.T) {
	modelServer := newOpenAILifecycleServer(t, "unused")
	defer modelServer.Close()
	fixture := newAwaitingAgentFixture(t, &mutableLifecycleConfigProvider{cfg: settings.LLM{
		Provider: settings.LLMProviderOpenAI, APIKey: "fixture-key",
		ModelName: "fixture-model", BaseURL: modelServer.URL,
	}})

	_, err := fixture.service.ResumeAgent(context.Background(), fixture.user.UserID, fixture.thread.ThreadID, &adk.ResumeParams{
		Targets: map[string]any{"missing-interrupt": "approved"},
	})
	require.Error(t, err)
	assertAwaitingRunRetryable(t, fixture)
	assertSingleFailedPreparedResume(t, fixture)
}

func TestResumeBuildFailureLeavesCheckpointAndAwaitingRunRetryable(t *testing.T) {
	buildFailure := errors.New("fixture model construction failed")
	fixture := newAwaitingAgentFixture(t, llmConfigProviderFunc(func(context.Context) (settings.LLM, error) {
		return settings.LLM{}, buildFailure
	}))
	checkpoint := []byte("retryable-checkpoint")
	require.NoError(t, fixture.service.store.Set(context.Background(), fixture.thread.CheckpointKey, checkpoint))

	_, err := fixture.service.ResumeAgent(context.Background(), fixture.user.UserID, fixture.thread.ThreadID, &adk.ResumeParams{})
	require.ErrorIs(t, err, buildFailure)
	assertAwaitingRunRetryable(t, fixture)
	assertSingleFailedPreparedResume(t, fixture)
	stored, exists, err := fixture.service.store.Get(context.Background(), fixture.thread.CheckpointKey)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, checkpoint, stored)
}

func TestConcurrentPreparedResumeTransitionSelectsOneActiveRun(t *testing.T) {
	ctx := context.Background()
	fixture := newAwaitingAgentFixture(t, nil)
	prepared := make([]uuid.UUID, 2)
	for i := range prepared {
		var err error
		prepared[i], err = fixture.service.createPreparedRunRecord(ctx, fixture.thread)
		require.NoError(t, err)
	}

	errs := make([]error, len(prepared))
	var wg sync.WaitGroup
	for i, runID := range prepared {
		wg.Add(1)
		go func(index int, candidate uuid.UUID) {
			defer wg.Done()
			errs[index] = fixture.service.transitionAwaitingRun(ctx, fixture.thread, fixture.oldRunID, candidate)
		}(i, runID)
	}
	wg.Wait()

	var activated uuid.UUID
	for i, err := range errs {
		if err == nil {
			require.Equal(t, uuid.Nil, activated)
			activated = prepared[i]
			continue
		}
		require.ErrorIs(t, err, sql.ErrNoRows)
		require.NoError(t, fixture.catalog.Queries.FinishAgentRun(ctx, repo.FinishAgentRunParams{
			Status: "failed", UpdatedAt: dbtypes.NewTimestamp(time.Now()),
			RunID: prepared[i], UserID: fixture.user.UserID, ThreadID: fixture.thread.ThreadID,
		}))
	}
	require.NotEqual(t, uuid.Nil, activated)

	var activeRuns int
	require.NoError(t, fixture.catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM agent_runs
		WHERE user_id = ? AND thread_id = ? AND activation_state = 'active'
			AND status IN ('running', 'cancel_requested', 'awaiting_confirmation')
	`, fixture.user.UserID, fixture.thread.ThreadID).Scan(&activeRuns))
	require.Equal(t, 1, activeRuns)
	thread, err := fixture.catalog.Queries.GetAgentThread(ctx, repo.GetAgentThreadParams{
		UserID: fixture.user.UserID, ThreadID: fixture.thread.ThreadID,
	})
	require.NoError(t, err)
	require.Equal(t, activated, thread.ActiveRunID.UUID)
}

func newAwaitingAgentFixture(t *testing.T, provider LLMConfigProvider) awaitingAgentFixture {
	t.Helper()
	ctx := context.Background()
	catalogDir := t.TempDir()
	require.NoError(t, os.Chmod(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "agent-resume.sqlite3"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, catalog.Close(context.Background()))
	})
	require.NoError(t, catalog.Migrate(ctx))
	user, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "agent-resume-test", Password: "unused", DisplayName: "Agent Resume Test",
		Role: "user", WebauthnUserHandle: []byte("agent-resume-test-handle"),
	})
	require.NoError(t, err)
	libraries := NewAuthorizedLibraryFactory(catalog.Queries, nil, catalog.SQL)
	refStore := ref.NewMemoryStore(0, 0)
	service := NewAgentService(
		catalog.Queries, catalog.SQL, catalog.Writer, provider, refStore, libraries,
		NewConversationStore(0), "",
	).(*agentService)
	thread, err := service.EnsureThread(ctx, user.UserID, "resume-thread", "free", ThreadBindings{})
	require.NoError(t, err)
	oldRunID, err := service.createRun(ctx, thread)
	require.NoError(t, err)
	now := dbtypes.NewTimestamp(time.Now())
	require.NoError(t, catalog.Queries.FinishAgentRun(ctx, repo.FinishAgentRunParams{
		Status: "awaiting_confirmation", UpdatedAt: now, RunID: oldRunID,
		UserID: user.UserID, ThreadID: thread.ThreadID,
	}))
	require.NoError(t, catalog.Queries.FinishAgentThread(ctx, repo.FinishAgentThreadParams{
		Status: "awaiting_confirmation", RunID: uuid.NullUUID{UUID: oldRunID, Valid: true},
		UpdatedAt: now, UserID: user.UserID, ThreadID: thread.ThreadID,
	}))
	thread, err = catalog.Queries.GetAgentThread(ctx, repo.GetAgentThreadParams{
		UserID: user.UserID, ThreadID: thread.ThreadID,
	})
	require.NoError(t, err)
	return awaitingAgentFixture{catalog: catalog, service: service, user: user, thread: thread, oldRunID: oldRunID}
}

func getAgentRun(t *testing.T, fixture awaitingAgentFixture, runID uuid.UUID) repo.AgentRun {
	t.Helper()
	run, err := fixture.catalog.Queries.GetAgentRun(context.Background(), repo.GetAgentRunParams{
		RunID: runID, UserID: fixture.user.UserID, ThreadID: fixture.thread.ThreadID,
	})
	require.NoError(t, err)
	return run
}

func assertAwaitingRunRetryable(t *testing.T, fixture awaitingAgentFixture) {
	t.Helper()
	oldRun := getAgentRun(t, fixture, fixture.oldRunID)
	require.Equal(t, "awaiting_confirmation", oldRun.Status)
	require.Equal(t, "active", oldRun.ActivationState)
	thread, err := fixture.catalog.Queries.GetAgentThread(context.Background(), repo.GetAgentThreadParams{
		UserID: fixture.user.UserID, ThreadID: fixture.thread.ThreadID,
	})
	require.NoError(t, err)
	require.Equal(t, "awaiting_confirmation", thread.Status)
	require.Equal(t, fixture.oldRunID, thread.ActiveRunID.UUID)
}

func assertSingleFailedPreparedResume(t *testing.T, fixture awaitingAgentFixture) {
	t.Helper()
	var failed int
	require.NoError(t, fixture.catalog.SQL.QueryRowContext(context.Background(), `
		SELECT count(*) FROM agent_runs
		WHERE user_id = ? AND thread_id = ? AND run_id <> ?
			AND status = 'failed' AND activation_state = 'terminal'
	`, fixture.user.UserID, fixture.thread.ThreadID, fixture.oldRunID.String()).Scan(&failed))
	rows, err := fixture.catalog.SQL.QueryContext(context.Background(), `
		SELECT run_id, status, activation_state FROM agent_runs
		WHERE user_id = ? AND thread_id = ? ORDER BY created_at
	`, fixture.user.UserID, fixture.thread.ThreadID)
	require.NoError(t, err)
	defer rows.Close()
	var states []string
	for rows.Next() {
		var runID, status, activation string
		require.NoError(t, rows.Scan(&runID, &status, &activation))
		states = append(states, runID+":"+status+":"+activation)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, 1, failed, "runs: %v", states)
}
