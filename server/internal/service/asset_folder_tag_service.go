package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

// FolderSummary describes one repository-relative directory projected from
// the repository node graph. Counts include active Locations on descendants.
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

func optionalTimeFromSQLite(value any) *time.Time {
	var timestamp dbtypes.Timestamp
	if err := timestamp.Scan(value); err != nil || !timestamp.Valid {
		return nil
	}
	t := timestamp.Time
	return &t
}

func optionalStringFromSQLiteUUID(value any) *string {
	var id uuid.UUID
	switch typed := value.(type) {
	case uuid.UUID:
		id = typed
	case uuid.NullUUID:
		if !typed.Valid {
			return nil
		}
		id = typed.UUID
	case string:
		parsed, err := uuid.Parse(typed)
		if err != nil {
			return nil
		}
		id = parsed
	case []byte:
		parsed, err := uuid.ParseBytes(typed)
		if err != nil {
			return nil
		}
		id = parsed
	default:
		return nil
	}
	if id == uuid.Nil {
		return nil
	}
	text := id.String()
	return &text
}

// ListFolderSummaries lists immediate child folders of parentPath, scoped by
// owner and optionally by repository. When repositoryID is nil, folders from
// every repository the owner can see are returned.
func (s *assetService) ListFolderSummaries(ctx context.Context, ownerID *int32, repositoryID *string, parentPath string) ([]FolderSummary, error) {
	var repoUUID uuid.NullUUID
	if repositoryID != nil && strings.TrimSpace(*repositoryID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*repositoryID))
		if err != nil {
			return nil, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	rows, err := s.readQueries.GetFolderChildSummaries(ctx, repo.GetFolderChildSummariesParams{
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
		repoID := row.RepositoryID
		if repoID == uuid.Nil {
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
			DateStart:      optionalTimeFromSQLite(row.DateStart),
			DateEnd:        optionalTimeFromSQLite(row.DateEnd),
			CoverAssetID:   optionalStringFromSQLiteUUID(row.CoverAssetID),
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
	row, err := s.readQueries.GetFolderSummary(ctx, repo.GetFolderSummaryParams{
		OwnerID:      ownerID,
		RepositoryID: parsed,
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
		DateStart:      optionalTimeFromSQLite(row.DateStart),
		DateEnd:        optionalTimeFromSQLite(row.DateEnd),
		CoverAssetID:   optionalStringFromSQLiteUUID(row.CoverAssetID),
	}, nil
}

// ListTagSummaries lists the tag vocabulary (manual and AI/system) visible to
// the owner, with usage counts and covers, optionally filtered by source or
// name substring.
func (s *assetService) ListTagSummaries(ctx context.Context, ownerID *int32, repositoryID *string, source *string, query *string, limit, offset int) ([]TagSummary, error) {
	var repoUUID uuid.NullUUID
	if repositoryID != nil && strings.TrimSpace(*repositoryID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*repositoryID))
		if err != nil {
			return nil, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	rows, err := s.readQueries.GetTagSummaries(ctx, repo.GetTagSummariesParams{
		OwnerID:      ownerID,
		RepositoryID: repoUUID,
		Source:       source,
		Query:        query,
		Offset:       int64(offset),
		Limit:        int64(limit),
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
			CoverAssetID: optionalStringFromSQLiteUUID(row.CoverAssetID),
			LastUsedAt:   optionalTimeFromSQLite(row.LastUsedAt),
		})
	}
	return summaries, nil
}

// repositoryNamesByID builds a repo_id -> name lookup for enriching folder
// summaries without an absolute path leak (repositories.path is never read here).
func (s *assetService) repositoryNamesByID(ctx context.Context) (map[string]string, error) {
	repos, err := s.readQueries.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	names := make(map[string]string, len(repos))
	for _, r := range repos {
		if r.RepoID != uuid.Nil {
			names[r.RepoID.String()] = r.Name
		}
	}
	return names, nil
}
