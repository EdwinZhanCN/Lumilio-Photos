package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"server/internal/agent/facets"
	"server/internal/agent/ref"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/search"

	"github.com/google/uuid"
)

// AuthorizedLibraryFactory is the only construction boundary for Agent
// data-plane access. Every request receives a library permanently bound to a
// non-optional owner id; tools never receive bare *repo.Queries.
type AuthorizedLibraryFactory struct {
	queries *repo.Queries
	search  RetrieverSearch
}

func NewAuthorizedLibraryFactory(queries *repo.Queries, search RetrieverSearch) *AuthorizedLibraryFactory {
	return &AuthorizedLibraryFactory{queries: queries, search: search}
}

func (f *AuthorizedLibraryFactory) ForUser(userID int32) *AuthorizedLibrary {
	return &AuthorizedLibrary{userID: userID, queries: f.queries, search: f.search}
}

func (f *AuthorizedLibraryFactory) AuthorizeAssetIDs(ctx context.Context, userID int32, ids []uuid.UUID) ([]uuid.UUID, error) {
	return f.ForUser(userID).AuthorizeAssetIDs(ctx, userID, ids)
}

// AuthorizedLibrary is a user-bound facade over every read path used by the
// Agent, ref hydration, injection, and live-pin replay.
type AuthorizedLibrary struct {
	userID  int32
	queries *repo.Queries
	search  RetrieverSearch
}

func (l *AuthorizedLibrary) UserID() int32 { return l.userID }

func uuidSet(ids []uuid.UUID) map[uuid.UUID]struct{} {
	out := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

// AuthorizeAssetIDs is the membership assertion used by RefStore.Create and
// every ref consumer. It preserves input order and rejects the whole set if
// any member is missing, deleted, or owned by another user.
func (l *AuthorizedLibrary) AuthorizeAssetIDs(ctx context.Context, userID int32, ids []uuid.UUID) ([]uuid.UUID, error) {
	if userID != l.userID {
		return nil, sql.ErrNoRows
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := l.queries.GetAuthorizedAssetIDs(ctx, repo.GetAuthorizedAssetIDsParams{
		AssetIds: ids,
		OwnerID:  &l.userID,
	})
	if err != nil {
		return nil, err
	}
	allowed := uuidSet(rows)
	if len(allowed) != len(ids) {
		return nil, sql.ErrNoRows
	}
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		if _, ok := allowed[id]; !ok {
			return nil, sql.ErrNoRows
		}
		out[i] = id
	}
	return out, nil
}

func (l *AuthorizedLibrary) FilterAssetIDs(ctx context.Context, params repo.GetAssetIDsUnifiedParams) ([]uuid.UUID, error) {
	params.OwnerID = &l.userID
	if albumID, ok := int32Value(params.AlbumID); ok {
		if _, err := l.Album(ctx, albumID); err != nil {
			return nil, sql.ErrNoRows
		}
	}
	return l.queries.GetAssetIDsUnified(ctx, params)
}

func (l *AuthorizedLibrary) SearchSemantic(ctx context.Context, query string, strictness search.SetStrictness, maxResults int) ([]uuid.UUID, search.SetMeta, error) {
	if l.search == nil {
		return nil, search.SetMeta{}, errors.New("search unavailable")
	}
	return l.search.SearchAssetIDsSemanticForOwner(ctx, l.userID, query, strictness, maxResults)
}

func (l *AuthorizedLibrary) SearchOCR(ctx context.Context, query string, maxResults int) ([]uuid.UUID, error) {
	if l.search == nil {
		return nil, errors.New("search unavailable")
	}
	return l.search.SearchAssetIDsOCRForOwner(ctx, l.userID, query, maxResults)
}

func (l *AuthorizedLibrary) SearchPeople(ctx context.Context, personIDs []int32, limit int32) ([]uuid.UUID, error) {
	return l.queries.GetAssetIDsByPersonIDs(ctx, repo.GetAssetIDsByPersonIDsParams{
		UserID: &l.userID, PersonIds: personIDs, Limit: int64(limit),
	})
}

func (l *AuthorizedLibrary) LookupPeople(ctx context.Context, query *string, limit int32) ([]repo.AgentLookupPeopleRow, error) {
	return l.queries.AgentLookupPeople(ctx, repo.AgentLookupPeopleParams{
		UserID: &l.userID, NameQuery: query, Limit: int64(limit),
	})
}

func (l *AuthorizedLibrary) LookupAlbums(ctx context.Context, query *string, limit int32) ([]repo.AgentLookupAlbumsRow, error) {
	return l.queries.AgentLookupAlbums(ctx, repo.AgentLookupAlbumsParams{
		UserID: &l.userID, TitleQuery: query, Limit: int64(limit),
	})
}

func (l *AuthorizedLibrary) Album(ctx context.Context, albumID int32) (repo.Album, error) {
	album, err := l.queries.GetAlbumByID(ctx, albumID)
	if err != nil || album.UserID != l.userID {
		return repo.Album{}, sql.ErrNoRows
	}
	return album, nil
}

func (l *AuthorizedLibrary) Person(ctx context.Context, personID int32) (repo.GetPersonByIDScopedRow, error) {
	return l.queries.GetPersonByIDScoped(ctx, repo.GetPersonByIDScopedParams{
		ClusterID: personID,
		OwnerID:   &l.userID,
	})
}

func (l *AuthorizedLibrary) Assets(ctx context.Context, ids []uuid.UUID) ([]repo.Asset, error) {
	if _, err := l.AuthorizeAssetIDs(ctx, l.userID, ids); err != nil {
		return nil, err
	}
	return l.queries.GetAssetsByIDsForOwner(ctx, repo.GetAssetsByIDsForOwnerParams{
		AssetIds: ids, OwnerID: &l.userID,
	})
}

func (l *AuthorizedLibrary) BuildFacets(ctx context.Context, r *ref.Ref) (*ref.FacetSummary, error) {
	if _, err := l.AuthorizeAssetIDs(ctx, l.userID, r.AssetIDs); err != nil {
		return nil, err
	}
	return facets.Build(ctx, l.queries, r)
}

func (l *AuthorizedLibrary) AestheticScores(ctx context.Context, ids []uuid.UUID) ([]repo.AgentAssetAestheticScoresRow, error) {
	return l.queries.AgentAssetAestheticScores(ctx, repo.AgentAssetAestheticScoresParams{
		AssetIds: ids, UserID: &l.userID,
	})
}

func (l *AuthorizedLibrary) InspectAssets(ctx context.Context, ids []uuid.UUID) ([]repo.AgentInspectAssetsRow, error) {
	return l.queries.AgentInspectAssets(ctx, repo.AgentInspectAssetsParams{
		AssetIds: ids, UserID: &l.userID,
	})
}

func (l *AuthorizedLibrary) PeekAssets(ctx context.Context, ids []uuid.UUID) ([]repo.AgentPeekAssetsRow, error) {
	return l.queries.AgentPeekAssets(ctx, repo.AgentPeekAssetsParams{
		AssetIds: ids, UserID: &l.userID,
	})
}

func (l *AuthorizedLibrary) RankByTime(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	return l.queries.AgentRankAssetIDsByTime(ctx, repo.AgentRankAssetIDsByTimeParams{
		AssetIds: ids, UserID: &l.userID,
	})
}

func (l *AuthorizedLibrary) RankByQuality(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	return l.queries.RankAssetIDsByQuality(ctx, repo.RankAssetIDsByQualityParams{
		AssetIds: ids, UserID: &l.userID,
	})
}

func (l *AuthorizedLibrary) CapturedTimes(ctx context.Context, ids []uuid.UUID) ([]dbtypes.Timestamp, error) {
	return l.queries.AgentCapturedTimes(ctx, repo.AgentCapturedTimesParams{
		AssetIds: ids, UserID: &l.userID,
	})
}

func (l *AuthorizedLibrary) PHashEmbeddings(ctx context.Context, ids []uuid.UUID) ([]repo.GetPHashEmbeddingsByAssetIDsRow, error) {
	authorized, err := l.AuthorizeAssetIDs(ctx, l.userID, ids)
	if err != nil {
		return nil, err
	}
	return l.queries.GetPHashEmbeddingsByAssetIDs(ctx, authorized)
}

func (l *AuthorizedLibrary) String() string {
	return fmt.Sprintf("authorized-library(user=%d)", l.userID)
}

func int32Value(value any) (int32, bool) {
	switch typed := value.(type) {
	case int32:
		return typed, true
	case int64:
		return int32(typed), true
	case int:
		return int32(typed), true
	case *int32:
		if typed != nil {
			return *typed, true
		}
	case *int:
		if typed != nil {
			return int32(*typed), true
		}
	}
	return 0, false
}
