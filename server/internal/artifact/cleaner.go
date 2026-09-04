package artifact

import (
	"context"
	"errors"
	"fmt"
	"time"

	"server/internal/db"
	"server/internal/db/repo"
	"server/internal/storage"
)

// Cleaner is the catalog-driven owner of artifact reclamation.  It never
// guesses from filenames: thumbnail rows and the applied transcode stage are
// the complete catalog references for the canonical artifact subtree.
type Cleaner struct {
	database *db.DB
	files    *storage.RepositoryFSFactory
	grace    time.Duration
	now      func() time.Time
}

func NewCleaner(database *db.DB, files *storage.RepositoryFSFactory, grace time.Duration) (*Cleaner, error) {
	if database == nil || files == nil {
		return nil, fmt.Errorf("artifact cleaner requires catalog and repository filesystem")
	}
	if grace < 0 {
		return nil, fmt.Errorf("artifact cleanup grace must be non-negative")
	}
	return &Cleaner{database: database, files: files, grace: grace, now: time.Now}, nil
}

// Run keeps cleanup best-effort and cancellable.  A failed repository is
// retried on the next pass and never prevents cleanup of the remaining ones.
func (c *Cleaner) Run(ctx context.Context, interval time.Duration, report func(error)) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	run := func() {
		if err := c.RunOnce(ctx); err != nil && report != nil && ctx.Err() == nil {
			report(err)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (c *Cleaner) RunOnce(ctx context.Context) error {
	if c == nil || c.database == nil || c.files == nil {
		return fmt.Errorf("artifact cleaner is not configured")
	}
	repositories, err := c.database.ReaderQueries.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("list repositories for artifact cleanup: %w", err)
	}
	var runErr error
	for _, repository := range repositories {
		if err := c.cleanRepository(ctx, repository); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}
	return runErr
}

func (c *Cleaner) cleanRepository(ctx context.Context, repository repo.Repository) error {
	referenced, err := c.references(ctx, repository)
	if err != nil {
		return err
	}
	files, err := c.files.OpenContext(ctx, repository)
	if err != nil {
		return fmt.Errorf("open repository %s for artifact cleanup: %w", repository.RepoID, err)
	}
	defer files.Close()
	store, err := NewStore(files)
	if err != nil {
		return err
	}
	if _, err := store.RemoveOrphans(ctx, referenced, c.grace, c.now()); err != nil {
		return fmt.Errorf("remove orphan artifacts in repository %s: %w", repository.RepoID, err)
	}
	return nil
}

func (c *Cleaner) references(ctx context.Context, repository repo.Repository) (map[string]struct{}, error) {
	referenced := make(map[string]struct{})
	thumbnails, err := c.database.ReaderSQL.QueryContext(ctx, `
		SELECT storage_path FROM thumbnails WHERE repository_id = ?`, repository.RepoID.String())
	if err != nil {
		return nil, fmt.Errorf("read thumbnail artifact references: %w", err)
	}
	for thumbnails.Next() {
		var storagePath string
		if err := thumbnails.Scan(&storagePath); err != nil {
			thumbnails.Close()
			return nil, err
		}
		referenced[storagePath] = struct{}{}
	}
	if err := thumbnails.Close(); err != nil {
		return nil, err
	}

	transcodes, err := c.database.ReaderSQL.QueryContext(ctx, `
		SELECT DISTINCT a.content_id, state.pipeline_version, a.type
		FROM asset_pipeline_state state
		JOIN assets a ON a.asset_id = state.asset_id
		JOIN active_asset_occurrences occurrence ON occurrence.asset_id = a.asset_id
		WHERE occurrence.repository_id = ?
		  AND state.stage = 'transcode'
		  AND state.applied_version = state.desired_version
		  AND state.terminal_error IS NULL`, repository.RepoID.String())
	if err != nil {
		return nil, fmt.Errorf("read transcode artifact references: %w", err)
	}
	defer transcodes.Close()
	for transcodes.Next() {
		var contentID, pipelineVersion, assetType string
		if err := transcodes.Scan(&contentID, &pipelineVersion, &assetType); err != nil {
			return nil, err
		}
		name := ""
		switch assetType {
		case "VIDEO":
			name = "web.mp4"
		case "AUDIO":
			name = "web.mp3"
		}
		if name == "" {
			continue
		}
		artifactPath, err := (Identity{SourceFence: contentID, Stage: "transcode", PipelineVersion: pipelineVersion, Name: name}).Path()
		if err != nil {
			return nil, err
		}
		referenced[artifactPath.String()] = struct{}{}
	}
	if err := transcodes.Err(); err != nil {
		return nil, err
	}
	return referenced, nil
}
