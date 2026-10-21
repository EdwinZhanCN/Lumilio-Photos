package core

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"strings"
)

const MaxMusicSelection = 100

type MusicSearch struct {
	RefID                 string  `json:"ref_id,omitempty" jsonschema:"description=Optional existing selection ref to refine; always use the latest attached selection when refining"`
	Query                 string  `json:"query,omitempty"`
	Artist                string  `json:"artist,omitempty"`
	ExcludeArtist         string  `json:"exclude_artist,omitempty"`
	Genre                 string  `json:"genre,omitempty"`
	LikedOnly             bool    `json:"liked_only,omitempty"`
	MinRating             int     `json:"min_rating,omitempty" jsonschema:"minimum=0,maximum=5"`
	MissingCover          bool    `json:"missing_cover,omitempty"`
	MissingLyrics         bool    `json:"missing_lyrics,omitempty"`
	DurationBudgetSeconds float64 `json:"duration_budget_seconds,omitempty" jsonschema:"description=Maximum total duration; excludes tracks with unknown duration"`
	DiverseArtists        bool    `json:"diverse_artists,omitempty" jsonschema:"description=Round-robin artist ordering before selection"`
	Limit                 int     `json:"limit,omitempty" jsonschema:"minimum=1,maximum=100"`
}

func (l *AuthorizedLibrary) SearchMusic(ctx context.Context, in MusicSearch, scope []uuid.UUID) ([]repo.SearchAgentMusicRow, bool, error) {
	if in.MinRating < 0 || in.MinRating > 5 || in.DurationBudgetSeconds < 0 || in.Limit < 0 || in.Limit > MaxMusicSelection {
		return nil, false, errors.New("invalid music limits")
	}
	if scope != nil {
		if _, err := l.AuthorizeAssetIDs(ctx, l.userID, scope); err != nil {
			return nil, false, err
		}
	}
	rows, err := l.queries.SearchAgentMusic(ctx, repo.SearchAgentMusicParams{OwnerID: l.userID, AssetIds: dbtypes.UUIDsJSONParam(scope), Scoped: scope != nil, Query: in.Query, Artist: in.Artist, ExcludeArtist: in.ExcludeArtist, Genre: in.Genre, LikedOnly: in.LikedOnly, MinRating: in.MinRating, MissingCover: in.MissingCover, MissingLyrics: in.MissingLyrics})
	if err != nil {
		return nil, false, err
	}
	truncated := len(rows) > 2000
	if truncated {
		rows = rows[:2000]
	}
	if scope != nil {
		byID := make(map[uuid.UUID]repo.SearchAgentMusicRow, len(rows))
		for _, row := range rows {
			byID[row.TrackID] = row
		}
		rows = nil
		for _, id := range scope {
			if row, ok := byID[id]; ok {
				rows = append(rows, row)
			}
		}
	}
	selected, limited := selectMusic(rows, in)
	return selected, truncated || limited, nil
}

func selectMusic(rows []repo.SearchAgentMusicRow, in MusicSearch) ([]repo.SearchAgentMusicRow, bool) {
	if in.DiverseArtists {
		groups := map[string][]repo.SearchAgentMusicRow{}
		artists := []string{}
		for _, row := range rows {
			key := strings.ToLower(strings.TrimSpace(row.ArtistName))
			if _, ok := groups[key]; !ok {
				artists = append(artists, key)
			}
			groups[key] = append(groups[key], row)
		}
		ordered := make([]repo.SearchAgentMusicRow, 0, len(rows))
		for len(ordered) < len(rows) {
			for _, artist := range artists {
				if len(groups[artist]) > 0 {
					ordered = append(ordered, groups[artist][0])
					groups[artist] = groups[artist][1:]
				}
			}
		}
		rows = ordered
	}
	limit := in.Limit
	if limit == 0 {
		limit = 30
	}
	out := make([]repo.SearchAgentMusicRow, 0, min(limit, len(rows)))
	duration := 0.0
	for _, row := range rows {
		if len(out) >= limit {
			break
		}
		if in.DurationBudgetSeconds > 0 {
			if row.Duration == nil || *row.Duration <= 0 || duration+*row.Duration > in.DurationBudgetSeconds {
				continue
			}
			duration += *row.Duration
		}
		out = append(out, row)
	}
	return out, len(out) < len(rows)
}

// MusicTracks rejects partial snapshots and restores the exact authorized order.
func (l *AuthorizedLibrary) MusicTracks(ctx context.Context, ids []uuid.UUID) ([]repo.SearchAgentMusicRow, error) {
	if len(ids) > MaxMusicSelection {
		return nil, errors.New("music snapshot too large")
	}
	rows, _, err := l.SearchMusic(ctx, MusicSearch{Limit: MaxMusicSelection}, ids)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, sql.ErrNoRows
	}
	return rows, nil
}

func (l *AuthorizedLibrary) LookupMusicPlaylists(ctx context.Context, query string) ([]repo.LookupAgentMusicPlaylistsRow, error) {
	return l.queries.LookupAgentMusicPlaylists(ctx, repo.LookupAgentMusicPlaylistsParams{OwnerID: l.userID, Query: &query})
}
func (l *AuthorizedLibrary) MusicPlaylist(ctx context.Context, id uuid.UUID) (repo.GetMusicPlaylistRow, error) {
	return l.queries.GetMusicPlaylist(ctx, repo.GetMusicPlaylistParams{OwnerID: l.userID, PlaylistID: id})
}
