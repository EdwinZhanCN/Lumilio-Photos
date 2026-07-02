package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// FolderSummary describes one repository-relative folder derived from
// assets.storage_path (there is no folders table). Counts are recursive
// over descendants.
type FolderSummary struct {
	RepositoryID   string
	RepositoryName string
	FolderPath     string
	DisplayName    string
	Depth          int
	AssetCount     int64
	PhotoCount     int64
	VideoCount     int64
	AudioCount     int64
	DateStart      *time.Time
	DateEnd        *time.Time
	CoverAssetID   *string
}

// TagSummary describes one (tag, source) pair's usage across the caller's
// accessible asset set.
type TagSummary struct {
	TagID        int32
	TagName      string
	Source       string
	AssetCount   int64
	CoverAssetID *string
	LastUsedAt   *time.Time
}

func folderDepth(folderPath string) int {
	if folderPath == "" {
		return 0
	}
	return strings.Count(folderPath, "/") + 1
}

func folderDisplayName(folderPath string) string {
	if folderPath == "" {
		return ""
	}
	segments := strings.Split(folderPath, "/")
	return segments[len(segments)-1]
}

func joinFolderPath(parentPath, childName string) string {
	if parentPath == "" {
		return childName
	}
	return parentPath + "/" + childName
}

func optionalTimeFromPg(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func optionalStringFromPgUUID(id pgtype.UUID) *string {
	parsed, ok := uuidFromPgUUID(id)
	if !ok {
		return nil
	}
	s := parsed.String()
	return &s
}

// ListFolderSummaries lists immediate child folders of parentPath, scoped by
// owner and optionally by repository. When repositoryID is nil, folders from
// every repository the owner can see are returned.
func (s *assetService) ListFolderSummaries(ctx context.Context, ownerID *int32, repositoryID *string, parentPath string) ([]FolderSummary, error) {
	var repoUUID pgtype.UUID
	if repositoryID != nil && strings.TrimSpace(*repositoryID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*repositoryID))
		if err != nil {
			return nil, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	rows, err := s.queries.GetFolderChildSummaries(ctx, repo.GetFolderChildSummariesParams{
		ParentPath:   parentPath,
		OwnerID:      ownerID,
		RepositoryID: repoUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("list folder summaries: %w", err)
	}
	if len(rows) == 0 {
		return []FolderSummary{}, nil
	}

	repoNames, err := s.repositoryNamesByID(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]FolderSummary, 0, len(rows))
	for _, row := range rows {
		repoID, ok := uuidFromPgUUID(row.RepositoryID)
		if !ok {
			continue
		}
		folderPath := joinFolderPath(parentPath, row.ChildName)
		summaries = append(summaries, FolderSummary{
			RepositoryID:   repoID.String(),
			RepositoryName: repoNames[repoID.String()],
			FolderPath:     folderPath,
			DisplayName:    row.ChildName,
			Depth:          folderDepth(folderPath),
			AssetCount:     row.AssetCount,
			PhotoCount:     row.PhotoCount,
			VideoCount:     row.VideoCount,
			AudioCount:     row.AudioCount,
			DateStart:      optionalTimeFromPg(row.DateStart),
			DateEnd:        optionalTimeFromPg(row.DateEnd),
			CoverAssetID:   optionalStringFromPgUUID(row.CoverAssetID),
		})
	}
	return summaries, nil
}

// GetFolderSummary returns aggregate stats for exactly one folder path
// (recursive descendants), used for the folder detail header.
func (s *assetService) GetFolderSummary(ctx context.Context, ownerID *int32, repositoryID string, folderPath string) (FolderSummary, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(repositoryID))
	if err != nil {
		return FolderSummary{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	repoUUID := pgtype.UUID{Bytes: parsed, Valid: true}

	row, err := s.queries.GetFolderSummary(ctx, repo.GetFolderSummaryParams{
		OwnerID:      ownerID,
		RepositoryID: repoUUID,
		FolderPath:   folderPath,
	})
	if err != nil {
		return FolderSummary{}, fmt.Errorf("get folder summary: %w", err)
	}

	repoNames, err := s.repositoryNamesByID(ctx)
	if err != nil {
		return FolderSummary{}, err
	}

	return FolderSummary{
		RepositoryID:   parsed.String(),
		RepositoryName: repoNames[parsed.String()],
		FolderPath:     folderPath,
		DisplayName:    folderDisplayName(folderPath),
		Depth:          folderDepth(folderPath),
		AssetCount:     row.AssetCount,
		PhotoCount:     row.PhotoCount,
		VideoCount:     row.VideoCount,
		AudioCount:     row.AudioCount,
		DateStart:      optionalTimeFromPg(row.DateStart),
		DateEnd:        optionalTimeFromPg(row.DateEnd),
		CoverAssetID:   optionalStringFromPgUUID(row.CoverAssetID),
	}, nil
}

// ListTagSummaries lists the tag vocabulary (manual and AI/system) visible to
// the owner, with usage counts and covers, optionally filtered by source or
// name substring.
func (s *assetService) ListTagSummaries(ctx context.Context, ownerID *int32, repositoryID *string, source *string, query *string, limit, offset int) ([]TagSummary, error) {
	var repoUUID pgtype.UUID
	if repositoryID != nil && strings.TrimSpace(*repositoryID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*repositoryID))
		if err != nil {
			return nil, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	rows, err := s.queries.GetTagSummaries(ctx, repo.GetTagSummariesParams{
		OwnerID:      ownerID,
		RepositoryID: repoUUID,
		Source:       source,
		Query:        query,
		Offset:       int32(offset),
		Limit:        int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list tag summaries: %w", err)
	}

	summaries := make([]TagSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, TagSummary{
			TagID:        row.TagID,
			TagName:      row.TagName,
			Source:       row.Source,
			AssetCount:   row.AssetCount,
			CoverAssetID: optionalStringFromPgUUID(row.CoverAssetID),
			LastUsedAt:   optionalTimeFromPg(row.LastUsedAt),
		})
	}
	return summaries, nil
}

// repositoryNamesByID builds a repo_id -> name lookup for enriching folder
// summaries without an absolute path leak (repositories.path is never read here).
func (s *assetService) repositoryNamesByID(ctx context.Context) (map[string]string, error) {
	repos, err := s.queries.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	names := make(map[string]string, len(repos))
	for _, r := range repos {
		if id, ok := uuidFromPgUUID(r.RepoID); ok {
			names[id.String()] = r.Name
		}
	}
	return names, nil
}
