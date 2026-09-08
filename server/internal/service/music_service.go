package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

const (
	defaultMusicPageSize = 50
	maxMusicPageSize     = 200
	maxPlaybackEntries   = 10000
	playbackSessionTTL   = 30 * time.Minute
)

var (
	ErrMusicNotFound    = errors.New("music item not found")
	ErrMusicConflict    = errors.New("music item changed; refresh and retry")
	ErrMusicInvalid     = errors.New("invalid music request")
	ErrPlaybackExpired  = errors.New("playback session expired")
	ErrPlaybackTooLarge = errors.New("playback result exceeds the session limit")
	ErrMusicUnavailable = errors.New("music track is unavailable")
)

// MusicCredit is an ordered, owner-scoped credit. A display name is never
// split on punctuation; one embedded credit remains one credit.
type MusicCredit struct {
	ArtistID    uuid.UUID `json:"artist_id"`
	DisplayName string    `json:"display_name"`
	Position    int64     `json:"position"`
	Role        string    `json:"role"`
}

type MusicOverride struct {
	Field   string  `json:"field"`
	Value   *string `json:"value,omitempty"`
	Present bool    `json:"present"`
}

type MusicTrack struct {
	TrackID                 uuid.UUID       `json:"track_id"`
	OwnerID                 int32           `json:"owner_id"`
	Designation             string          `json:"designation"`
	AlbumID                 *uuid.UUID      `json:"album_id,omitempty"`
	Title                   string          `json:"title"`
	AlbumTitle              string          `json:"album_title"`
	ArtistName              string          `json:"artist_name"`
	AlbumArtistName         string          `json:"album_artist_name"`
	Genre                   string          `json:"genre"`
	ReleaseDate             *string         `json:"release_date,omitempty"`
	ReleasePrecision        string          `json:"release_precision"`
	Edition                 string          `json:"edition"`
	ReleaseIdentifier       string          `json:"release_identifier"`
	DiscNumber              *int64          `json:"disc_number,omitempty"`
	DiscTotal               *int64          `json:"disc_total,omitempty"`
	TrackNumber             *int64          `json:"track_number,omitempty"`
	TrackTotal              *int64          `json:"track_total,omitempty"`
	Compilation             bool            `json:"compilation"`
	ExtractedSourceRevision int64           `json:"extracted_source_revision"`
	Revision                int64           `json:"revision"`
	OriginalFilename        string          `json:"original_filename"`
	MimeType                string          `json:"mime_type"`
	Duration                *float64        `json:"duration,omitempty"`
	TakenAt                 *time.Time      `json:"taken_at,omitempty"`
	IsDeleted               bool            `json:"is_deleted"`
	Liked                   bool            `json:"liked"`
	Artists                 []MusicCredit   `json:"artists,omitempty"`
	Overrides               []MusicOverride `json:"overrides,omitempty"`
}

type MusicTrackPage struct {
	Items  []MusicTrack `json:"items"`
	Total  int64        `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
}

type MusicAlbum struct {
	Favorite          bool          `json:"favorite"`
	AlbumID           uuid.UUID     `json:"album_id"`
	OwnerID           int32         `json:"owner_id"`
	Title             string        `json:"title"`
	ReleaseDate       *string       `json:"release_date,omitempty"`
	ReleasePrecision  string        `json:"release_precision"`
	Edition           string        `json:"edition"`
	ReleaseIdentifier string        `json:"release_identifier"`
	CoverAssetID      *uuid.UUID    `json:"cover_asset_id,omitempty"`
	Revision          int64         `json:"revision"`
	TrackCount        int64         `json:"track_count"`
	Artists           []MusicCredit `json:"artists,omitempty"`
	Tracks            []MusicTrack  `json:"tracks,omitempty"`
	artistSource      string
}

type MusicAlbumPage struct {
	Items  []MusicAlbum `json:"items"`
	Total  int64        `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
}

type MusicArtist struct {
	Favorite    bool      `json:"favorite"`
	ArtistID    uuid.UUID `json:"artist_id"`
	OwnerID     int32     `json:"owner_id"`
	DisplayName string    `json:"display_name"`
	Revision    int64     `json:"revision"`
	TrackCount  int64     `json:"track_count"`
	AlbumCount  int64     `json:"album_count"`
}

type MusicArtistPage struct {
	Items  []MusicArtist `json:"items"`
	Total  int64         `json:"total"`
	Offset int           `json:"offset"`
	Limit  int           `json:"limit"`
}

type MusicPlaylist struct {
	PlaylistID  uuid.UUID `json:"playlist_id"`
	OwnerID     int32     `json:"owner_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Revision    int64     `json:"revision"`
	EntryCount  int64     `json:"entry_count"`
}

type MusicPlaylistPage struct {
	Items  []MusicPlaylist `json:"items"`
	Total  int64           `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
}

type MusicPlaylistEntry struct {
	EntryID        string      `json:"entry_id"`
	PlaylistID     uuid.UUID   `json:"playlist_id"`
	TrackID        *uuid.UUID  `json:"track_id,omitempty"`
	SavedTitle     string      `json:"saved_title"`
	Position       int64       `json:"position"`
	IdempotencyKey *string     `json:"idempotency_key,omitempty"`
	Available      bool        `json:"available"`
	Track          *MusicTrack `json:"track,omitempty"`
}

type MusicEntryPosition struct {
	EntryID  string `json:"entry_id"`
	Position int64  `json:"position"`
}

type PlaybackSource struct {
	Sort      string `json:"sort,omitempty"`
	ArtistID  string `json:"artist_id,omitempty"`
	Kind      string `json:"kind"`
	ID        string `json:"id,omitempty"`
	Query     string `json:"query,omitempty"`
	LikedOnly bool   `json:"liked_only,omitempty"`
}

type MusicPlaybackSession struct {
	SessionID      uuid.UUID `json:"session_id"`
	OwnerID        int32     `json:"owner_id"`
	SourceKind     string    `json:"source_kind"`
	SourceID       string    `json:"source_id"`
	SourceRevision int64     `json:"source_revision"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
	TotalEntries   int64     `json:"total_entries"`
}

type MusicPlaybackEntry struct {
	EntryID       string     `json:"entry_id"`
	Sequence      int64      `json:"sequence"`
	TrackID       *uuid.UUID `json:"track_id,omitempty"`
	SourceEntryID *string    `json:"source_entry_id,omitempty"`
	SavedTitle    string     `json:"saved_title"`
	Available     bool       `json:"available"`
	TrackTitle    string     `json:"track_title,omitempty"`
	TrackArtist   string     `json:"track_artist,omitempty"`
	TrackAlbum    string     `json:"track_album,omitempty"`
	MimeType      string     `json:"mime_type,omitempty"`
	Duration      *float64   `json:"duration,omitempty"`
}

type MusicPlaybackPage struct {
	Items  []MusicPlaybackEntry `json:"items"`
	Total  int64                `json:"total"`
	Offset int                  `json:"offset"`
	Limit  int                  `json:"limit"`
}

type MusicTrackPatch struct {
	Title            *string
	AlbumTitle       *string
	ArtistName       *string
	AlbumArtistName  *string
	Genre            *string
	ReleaseDate      *string
	ReleasePrecision *string
	Edition          *string
	DiscNumber       *int64
	DiscTotal        *int64
	TrackNumber      *int64
	TrackTotal       *int64
	Compilation      *bool
	Designation      *string
	AlbumID          *uuid.UUID
	Artists          *[]string
	AlbumArtists     *[]string
}

type MusicAlbumPatch struct {
	Favorite         *bool
	Title            *string
	ReleaseDate      *string
	ReleasePrecision *string
	Edition          *string
	CoverAssetID     *uuid.UUID
	ArtistNames      *[]string
}

type MusicArtistPatch struct {
	Favorite    *bool
	DisplayName *string
}

type MusicLyrics struct {
	Content  string
	Revision int64
}

type MusicService interface {
	GetLyrics(context.Context, int32, uuid.UUID) (MusicLyrics, error)
	UpdateLyrics(context.Context, int32, uuid.UUID, string, int64) (MusicLyrics, error)
	ListTracks(context.Context, int32, string, string, string, bool, int, int) (MusicTrackPage, error)
	GetTrack(context.Context, int32, uuid.UUID) (MusicTrack, error)
	UpdateTrack(context.Context, int32, uuid.UUID, MusicTrackPatch, int64) (MusicTrack, error)
	ResetTrackOverrides(context.Context, int32, uuid.UUID) (MusicTrack, error)
	SetTrackDesignation(context.Context, int32, uuid.UUID, string, int64) (MusicTrack, error)

	ListAlbums(context.Context, int32, string, bool, int, int) (MusicAlbumPage, error)
	GetAlbum(context.Context, int32, uuid.UUID) (MusicAlbum, error)
	CreateAlbum(context.Context, int32, string, *string, string, string, *uuid.UUID, []string) (MusicAlbum, error)
	UpdateAlbum(context.Context, int32, uuid.UUID, MusicAlbumPatch, int64) (MusicAlbum, error)
	AssignTrackAlbum(context.Context, int32, uuid.UUID, *uuid.UUID, int64) error

	ListArtists(context.Context, int32, string, bool, int, int) (MusicArtistPage, error)
	GetArtist(context.Context, int32, uuid.UUID) (MusicArtist, error)
	UpdateArtist(context.Context, int32, uuid.UUID, MusicArtistPatch, int64) (MusicArtist, error)

	ListPlaylists(context.Context, int32, int, int) (MusicPlaylistPage, error)
	GetPlaylist(context.Context, int32, uuid.UUID) (MusicPlaylist, error)
	CreatePlaylist(context.Context, int32, string, string) (MusicPlaylist, error)
	UpdatePlaylist(context.Context, int32, uuid.UUID, string, string, int64) (MusicPlaylist, error)
	DeletePlaylist(context.Context, int32, uuid.UUID) error
	ListPlaylistEntries(context.Context, int32, uuid.UUID) ([]MusicPlaylistEntry, error)
	AddPlaylistEntry(context.Context, int32, uuid.UUID, uuid.UUID, string, *string, int64) (MusicPlaylistEntry, error)
	RemovePlaylistEntry(context.Context, int32, uuid.UUID, string, int64) error
	ReorderPlaylist(context.Context, int32, uuid.UUID, []MusicEntryPosition, int64) error

	CreatePlaybackSession(context.Context, int32, PlaybackSource) (MusicPlaybackSession, error)
	ListPlaybackEntries(context.Context, int32, uuid.UUID, int, int) (MusicPlaybackPage, error)
	ExpirePlaybackSession(context.Context, int32, uuid.UUID) error
	BackfillAll(context.Context, int) error
}

type musicService struct {
	queries     *repo.Queries
	readQueries *repo.Queries
	writer      *catalogtx.Writer
}

func NewMusicService(queries, readQueries *repo.Queries, writer *catalogtx.Writer) MusicService {
	if readQueries == nil {
		readQueries = queries
	}
	return &musicService{queries: queries, readQueries: readQueries, writer: writer}
}

func (s *musicService) write(ctx context.Context, body func(*sql.Tx, *repo.Queries) error) error {
	if s.writer == nil || s.queries == nil {
		return errors.New("music catalog writer is unavailable")
	}
	return s.writer.Transact(ctx, catalogtx.OperationMusicMutation, nil, func(tx *sql.Tx) error {
		return body(tx, s.queries.WithTx(tx))
	})
}

func timestampTime(value dbtypes.Timestamp) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func nullUUIDPointer(value uuid.NullUUID) *uuid.UUID {
	if !value.Valid || value.UUID == uuid.Nil {
		return nil
	}
	result := value.UUID
	return &result
}

func uuidNull(value *uuid.UUID) uuid.NullUUID {
	if value == nil || *value == uuid.Nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *value, Valid: true}
}

func clampMusicPage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultMusicPageSize
	}
	if limit > maxMusicPageSize {
		limit = maxMusicPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func trackFromListRow(row repo.ListMusicTracksRow) MusicTrack {
	return MusicTrack{
		TrackID: row.TrackID, OwnerID: row.OwnerID, Designation: row.Designation,
		AlbumID: nullUUIDPointer(row.AlbumID), Title: row.Title, AlbumTitle: row.AlbumTitle,
		ArtistName: row.ArtistName, AlbumArtistName: row.AlbumArtistName, Genre: row.Genre,
		ReleaseDate: row.ReleaseDate, ReleasePrecision: row.ReleasePrecision, Edition: row.Edition,
		ReleaseIdentifier: row.ReleaseIdentifier, DiscNumber: row.DiscNumber, DiscTotal: row.DiscTotal,
		TrackNumber: row.TrackNumber, TrackTotal: row.TrackTotal, Compilation: row.IsCompilation != 0,
		ExtractedSourceRevision: row.ExtractedSourceRevision, Revision: row.Revision,
		OriginalFilename: row.OriginalFilename, MimeType: row.MimeType, Duration: row.Duration,
		TakenAt: timestampTime(row.TakenTime), IsDeleted: row.IsDeleted, Liked: row.Liked,
	}
}

func trackFromDetailRow(row repo.GetMusicTrackRow) MusicTrack {
	return MusicTrack{
		TrackID: row.TrackID, OwnerID: row.OwnerID, Designation: row.Designation,
		AlbumID: nullUUIDPointer(row.AlbumID), Title: row.Title, AlbumTitle: row.AlbumTitle,
		ArtistName: row.ArtistName, AlbumArtistName: row.AlbumArtistName, Genre: row.Genre,
		ReleaseDate: row.ReleaseDate, ReleasePrecision: row.ReleasePrecision, Edition: row.Edition,
		ReleaseIdentifier: row.ReleaseIdentifier, DiscNumber: row.DiscNumber, DiscTotal: row.DiscTotal,
		TrackNumber: row.TrackNumber, TrackTotal: row.TrackTotal, Compilation: row.IsCompilation != 0,
		ExtractedSourceRevision: row.ExtractedSourceRevision, Revision: row.Revision,
		OriginalFilename: row.OriginalFilename, MimeType: row.MimeType, Duration: row.Duration,
		TakenAt: timestampTime(row.TakenTime), IsDeleted: row.IsDeleted, Liked: row.Liked,
	}
}

func creditsFromTrackRows(rows []repo.ListMusicTrackArtistsRow) []MusicCredit {
	credits := make([]MusicCredit, 0, len(rows))
	for _, row := range rows {
		credits = append(credits, MusicCredit{ArtistID: row.ArtistID, DisplayName: row.DisplayName, Position: row.Position, Role: row.Role})
	}
	return credits
}

func creditsFromAlbumRows(rows []repo.ListMusicAlbumArtistsRow) []MusicCredit {
	credits := make([]MusicCredit, 0, len(rows))
	for _, row := range rows {
		credits = append(credits, MusicCredit{ArtistID: row.ArtistID, DisplayName: row.DisplayName, Position: row.Position, Role: row.Role})
	}
	return credits
}

func overrideValues(rows []repo.MusicTrackOverride) []MusicOverride {
	result := make([]MusicOverride, 0, len(rows))
	for _, row := range rows {
		result = append(result, MusicOverride{Field: row.Field, Value: row.Value, Present: row.IsPresent != 0})
	}
	return result
}

func (s *musicService) enrichTrack(ctx context.Context, ownerID int32, track MusicTrack) (MusicTrack, error) {
	artists, err := s.readQueries.ListMusicTrackArtists(ctx, repo.ListMusicTrackArtistsParams{TrackID: track.TrackID, OwnerID: ownerID})
	if err != nil {
		return MusicTrack{}, err
	}
	overrides, err := s.readQueries.ListMusicTrackOverrides(ctx, track.TrackID.String())
	if err != nil {
		return MusicTrack{}, err
	}
	track.Artists = creditsFromTrackRows(artists)
	track.Overrides = overrideValues(overrides)
	return track, nil
}

func (s *musicService) ListTracks(ctx context.Context, ownerID int32, query, sort, artistFilter string, likedOnly bool, limit, offset int) (MusicTrackPage, error) {
	limit, offset = clampMusicPage(limit, offset)
	switch sort {
	case "title", "artist", "album", "track":
	default:
		sort = ""
	}
	rows, err := s.readQueries.ListMusicTracks(ctx, repo.ListMusicTracksParams{
		OwnerID: ownerID, ArtistFilter: artistFilter, Query: query, SortOrder: sort, LikedOnly: boolInt(likedOnly), Limit: int64(limit), Offset: int64(offset),
	})
	if err != nil {
		return MusicTrackPage{}, err
	}
	total, err := s.readQueries.CountMusicTracks(ctx, repo.CountMusicTracksParams{OwnerID: ownerID, ArtistFilter: artistFilter, Query: query, LikedOnly: boolInt(likedOnly)})
	if err != nil {
		return MusicTrackPage{}, err
	}
	items := make([]MusicTrack, 0, len(rows))
	for _, row := range rows {
		track, enrichErr := s.enrichTrack(ctx, ownerID, trackFromListRow(row))
		if enrichErr != nil {
			return MusicTrackPage{}, enrichErr
		}
		items = append(items, track)
	}
	return MusicTrackPage{Items: items, Total: total, Offset: offset, Limit: limit}, nil
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func (s *musicService) GetTrack(ctx context.Context, ownerID int32, trackID uuid.UUID) (MusicTrack, error) {
	row, err := s.readQueries.GetMusicTrack(ctx, repo.GetMusicTrackParams{TrackID: trackID, OwnerID: ownerID})
	if err != nil {
		return MusicTrack{}, err
	}
	return s.enrichTrack(ctx, ownerID, trackFromDetailRow(row))
}

type musicTrackValues struct {
	Designation      string
	AlbumID          uuid.NullUUID
	Title            string
	AlbumTitle       string
	ArtistName       string
	AlbumArtistName  string
	Genre            string
	ReleaseDate      *string
	ReleasePrecision string
	Edition          string
	DiscNumber       *int64
	DiscTotal        *int64
	TrackNumber      *int64
	TrackTotal       *int64
	Compilation      int64
}

func musicValuesFromSource(source repo.MusicTrack) musicTrackValues {
	return musicTrackValues{
		Designation: source.Designation, AlbumID: source.AlbumID, Title: source.Title,
		AlbumTitle: source.AlbumTitle, ArtistName: source.ArtistName, AlbumArtistName: source.AlbumArtistName,
		Genre: source.Genre, ReleaseDate: source.ReleaseDate, ReleasePrecision: source.ReleasePrecision,
		Edition: source.Edition, DiscNumber: source.DiscNumber, DiscTotal: source.DiscTotal,
		TrackNumber: source.TrackNumber, TrackTotal: source.TrackTotal, Compilation: source.IsCompilation,
	}
}

func musicValuesFromDetail(row repo.GetMusicTrackRow) musicTrackValues {
	return musicTrackValues{
		Designation: row.Designation, AlbumID: row.AlbumID, Title: row.Title,
		AlbumTitle: row.AlbumTitle, ArtistName: row.ArtistName, AlbumArtistName: row.AlbumArtistName,
		Genre: row.Genre, ReleaseDate: row.ReleaseDate, ReleasePrecision: row.ReleasePrecision,
		Edition: row.Edition, DiscNumber: row.DiscNumber, DiscTotal: row.DiscTotal,
		TrackNumber: row.TrackNumber, TrackTotal: row.TrackTotal, Compilation: row.IsCompilation,
	}
}

func applyOverrides(values *musicTrackValues, overrides []repo.MusicTrackOverride) {
	for _, override := range overrides {
		value := ""
		if override.Value != nil {
			value = *override.Value
		}
		switch override.Field {
		case "title":
			values.Title = value
		case "album_title":
			values.AlbumTitle = value
		case "artist_name":
			values.ArtistName = value
		case "album_artist_name":
			values.AlbumArtistName = value
		case "genre":
			values.Genre = value
		case "release_date":
			if override.IsPresent == 0 {
				values.ReleaseDate = nil
			} else {
				values.ReleaseDate = stringPointer(value)
			}
		case "release_precision":
			if override.IsPresent != 0 {
				if precision, err := validateMusicReleasePrecision(value); err == nil {
					values.ReleasePrecision = precision
				}
			}
		case "edition":
			values.Edition = value
		case "designation":
			if value == "music" || value == "other" {
				values.Designation = value
			}
		case "album_id":
			if override.IsPresent == 0 {
				values.AlbumID = uuid.NullUUID{}
			} else if albumID, err := uuid.Parse(value); err == nil && albumID != uuid.Nil {
				values.AlbumID = uuidNull(&albumID)
			}
		case "disc_number":
			values.DiscNumber = overrideInt(value, override.IsPresent)
		case "disc_total":
			values.DiscTotal = overrideInt(value, override.IsPresent)
		case "track_number":
			values.TrackNumber = overrideInt(value, override.IsPresent)
		case "track_total":
			values.TrackTotal = overrideInt(value, override.IsPresent)
		case "compilation":
			values.Compilation = 0
			if override.IsPresent != 0 && value == "1" {
				values.Compilation = 1
			}
		}
	}
}

func overrideInt(value string, present int64) *int64 {
	if present == 0 || value == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return nil
	}
	return &parsed
}

func stringPointer(value string) *string {
	return &value
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func hasOverride(overrides []repo.MusicTrackOverride, field string) bool {
	for _, override := range overrides {
		if override.Field == field {
			return true
		}
	}
	return false
}

func effectiveAudioMetadata(metadata dbtypes.SpecificMetadata, filename string) (musicTrackValues, []string, []string, []string, []string, string, error) {
	parsed := dbtypes.AudioSpecificMetadata{}
	if len(metadata) > 0 {
		var err error
		parsed, err = metadata.UnmarshalAudio()
		if err != nil {
			return musicTrackValues{}, nil, nil, nil, nil, "", err
		}
	}
	title := strings.TrimSpace(parsed.Title)
	if title == "" {
		title = strings.TrimSpace(filename)
	}
	artists := append([]string(nil), parsed.Artists...)
	if len(artists) == 0 && strings.TrimSpace(parsed.Artist) != "" {
		artists = []string{strings.TrimSpace(parsed.Artist)}
	}
	albumArtists := append([]string(nil), parsed.AlbumArtists...)
	if len(albumArtists) == 0 && strings.TrimSpace(parsed.AlbumArtist) != "" {
		albumArtists = []string{strings.TrimSpace(parsed.AlbumArtist)}
	}
	releaseDate := strings.TrimSpace(parsed.ReleaseDate)
	precision := parsed.ReleasePrecision
	if releaseDate == "" && parsed.Year > 0 {
		releaseDate = fmt.Sprintf("%04d", parsed.Year)
		precision = "year"
	}
	if precision == "" {
		precision = "unknown"
	}
	compilation := int64(0)
	if parsed.Compilation != nil && *parsed.Compilation {
		compilation = 1
	}
	values := musicTrackValues{
		Designation: "music", Title: title, AlbumTitle: strings.TrimSpace(parsed.Album),
		ArtistName: firstString(artists), AlbumArtistName: firstString(albumArtists),
		Genre: strings.TrimSpace(parsed.Genre), ReleaseDate: optionalString(releaseDate),
		ReleasePrecision: precision, Edition: strings.TrimSpace(parsed.Edition),
		DiscNumber: intPointer64(parsed.DiscNumber), DiscTotal: intPointer64(parsed.DiscTotal),
		TrackNumber: intPointer64(parsed.TrackNumber), TrackTotal: intPointer64(parsed.TrackTotal),
		Compilation: compilation,
	}
	return values, artists, albumArtists, parsed.ArtistIDs, parsed.AlbumArtistIDs, strings.TrimSpace(parsed.ReleaseID), nil
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func intPointer64(value *int) *int64 {
	if value == nil {
		return nil
	}
	converted := int64(*value)
	return &converted
}

func jsonStringArray(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err
}

func upsertMusicArtistTx(ctx context.Context, q *repo.Queries, ownerID int32, name, externalID string) (uuid.UUID, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return uuid.Nil, nil
	}
	normalized := normalizeMusicName(name)
	candidates, err := q.FindMusicArtistsByNormalized(ctx, repo.FindMusicArtistsByNormalizedParams{OwnerID: ownerID, NormalizedName: normalized})
	if err != nil {
		return uuid.Nil, err
	}
	if externalID != "" {
		for _, candidate := range candidates {
			if candidate.ExternalID == externalID {
				return candidate.ArtistID, nil
			}
		}
		// An embedded ID that disagrees with a same-named artist is a distinct
		// identity. Never silently merge it through the name fallback.
	} else if len(candidates) == 1 {
		return candidates[0].ArtistID, nil
	}
	artist, err := q.CreateMusicArtist(ctx, repo.CreateMusicArtistParams{
		ArtistID: uuid.New(), OwnerID: ownerID, DisplayName: name,
		NormalizedName: normalized, ExternalID: externalID,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return artist.ArtistID, nil
}

func normalizeMusicName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func normalizeMusicCredits(names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func replaceTrackArtistsTx(ctx context.Context, q *repo.Queries, ownerID int32, trackID uuid.UUID, names, externalIDs []string) error {
	if err := q.DeleteMusicTrackArtists(ctx, repo.DeleteMusicTrackArtistsParams{TrackID: trackID, OwnerID: ownerID}); err != nil {
		return err
	}
	for position, name := range names {
		externalID := ""
		if position < len(externalIDs) {
			externalID = strings.TrimSpace(externalIDs[position])
		}
		artistID, err := upsertMusicArtistTx(ctx, q, ownerID, name, externalID)
		if err != nil {
			return err
		}
		if artistID == uuid.Nil {
			continue
		}
		if err := q.AddMusicTrackArtist(ctx, repo.AddMusicTrackArtistParams{
			TrackID: trackID, ArtistID: artistID, Position: int64(position), Role: "track_artist", OwnerID: ownerID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func replaceAlbumArtistsTx(ctx context.Context, q *repo.Queries, ownerID int32, albumID uuid.UUID, names, externalIDs []string) error {
	if err := q.DeleteMusicAlbumArtists(ctx, repo.DeleteMusicAlbumArtistsParams{AlbumID: albumID, OwnerID: ownerID}); err != nil {
		return err
	}
	for position, name := range names {
		externalID := ""
		if position < len(externalIDs) {
			externalID = strings.TrimSpace(externalIDs[position])
		}
		artistID, err := upsertMusicArtistTx(ctx, q, ownerID, name, externalID)
		if err != nil {
			return err
		}
		if artistID == uuid.Nil {
			continue
		}
		if err := q.AddMusicAlbumArtist(ctx, repo.AddMusicAlbumArtistParams{
			AlbumID: albumID, ArtistID: artistID, Position: int64(position), Role: "album_artist", OwnerID: ownerID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// SyncMusicTrackFromAssetTx is called by the metadata commit coordinator. It
// is intentionally transaction-scoped so extracted facts, effective values,
// and searchable credits become visible together.
func SyncMusicTrackFromAssetTx(ctx context.Context, tx *sql.Tx, q *repo.Queries, trackID uuid.UUID, metadata dbtypes.SpecificMetadata) error {
	if tx == nil || q == nil || trackID == uuid.Nil {
		return errors.New("music metadata sync requires transaction, queries, and asset")
	}
	var musicCatalogExists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'music_tracks'").Scan(&musicCatalogExists); errors.Is(err, sql.ErrNoRows) {
		// Metadata commits must remain compatible with catalogs created before
		// the music migration. The next startup backfill reconciles those rows.
		return nil
	} else if err != nil {
		return err
	}
	asset, err := q.GetAssetByIDAny(ctx, trackID)
	if err != nil {
		return err
	}
	if asset.Type != string(dbtypes.AssetTypeAudio) || asset.OwnerID == nil {
		return nil
	}
	ownerID := *asset.OwnerID
	values, artists, albumArtists, artistIDs, albumArtistIDs, releaseID, err := effectiveAudioMetadata(metadata, asset.OriginalFilename)
	if err != nil {
		return fmt.Errorf("decode audio metadata: %w", err)
	}
	source, sourceErr := q.GetMusicTrackSource(ctx, repo.GetMusicTrackSourceParams{TrackID: trackID, OwnerID: ownerID})
	if sourceErr != nil && !errors.Is(sourceErr, sql.ErrNoRows) {
		return sourceErr
	}
	overrides, err := q.ListMusicTrackOverrides(ctx, trackID.String())
	if err != nil {
		return err
	}
	if sourceErr == nil {
		values.AlbumID = source.AlbumID
		if source.Designation == "music" || source.Designation == "other" {
			values.Designation = source.Designation
		}
	}
	if !hasOverride(overrides, "artist_name") {
		values.ArtistName = firstString(artists)
	}
	if !hasOverride(overrides, "album_artist_name") {
		values.AlbumArtistName = firstString(albumArtists)
	}
	applyOverrides(&values, overrides)

	albumArtistsManual := false
	if !hasOverride(overrides, "album_id") && !values.AlbumID.Valid && releaseID != "" && values.AlbumTitle != "" {
		album, albumErr := q.GetMusicAlbumByIdentity(ctx, repo.GetMusicAlbumByIdentityParams{
			OwnerID: ownerID, Title: values.AlbumTitle, ReleaseIdentifier: releaseID,
		})
		if albumErr == nil {
			values.AlbumID = uuidNull(&album.AlbumID)
			albumArtistsManual = album.ArtistSource == "manual"
		} else if errors.Is(albumErr, sql.ErrNoRows) {
			created, createErr := q.CreateMusicAlbum(ctx, repo.CreateMusicAlbumParams{
				AlbumID: uuid.New(), OwnerID: ownerID, Title: values.AlbumTitle,
				ReleaseDate: values.ReleaseDate, ReleasePrecision: values.ReleasePrecision,
				Edition: values.Edition, ReleaseIdentifier: releaseID, SourceGroup: "",
				ArtistSource: "extracted",
				CoverAssetID: uuid.NullUUID{},
			})
			if createErr != nil {
				return createErr
			}
			values.AlbumID = uuidNull(&created.AlbumID)
			if err := replaceAlbumArtistsTx(ctx, q, ownerID, created.AlbumID, albumArtists, albumArtistIDs); err != nil {
				return err
			}
		} else {
			return albumErr
		}
	}

	extractedArtists, err := jsonStringArray(artists)
	if err != nil {
		return err
	}
	extractedAlbumArtists, err := jsonStringArray(albumArtists)
	if err != nil {
		return err
	}
	extractedArtistIDs, err := jsonStringArray(artistIDs)
	if err != nil {
		return err
	}
	extractedAlbumArtistIDs, err := jsonStringArray(albumArtistIDs)
	if err != nil {
		return err
	}
	sourceRevision := int64(0)
	if asset.UpdatedAt.Valid {
		sourceRevision = asset.UpdatedAt.Time.UnixMicro()
	}
	if sourceRevision <= 0 {
		sourceRevision = time.Now().UTC().UnixMicro()
	}
	updated, err := q.UpsertMusicTrack(ctx, repo.UpsertMusicTrackParams{
		TrackID: trackID, OwnerID: ownerID, Designation: values.Designation, AlbumID: values.AlbumID,
		Title: values.Title, AlbumTitle: values.AlbumTitle, ArtistName: values.ArtistName,
		AlbumArtistName: values.AlbumArtistName, Genre: values.Genre, ReleaseDate: values.ReleaseDate,
		ReleasePrecision: values.ReleasePrecision, Edition: values.Edition, ReleaseIdentifier: releaseID,
		DiscNumber: values.DiscNumber, DiscTotal: values.DiscTotal, TrackNumber: values.TrackNumber,
		TrackTotal: values.TrackTotal, IsCompilation: values.Compilation,
		ExtractedArtists: extractedArtists, ExtractedAlbumArtists: extractedAlbumArtists,
		ExtractedArtistIds: extractedArtistIDs, ExtractedAlbumArtistIds: extractedAlbumArtistIDs,
		ExtractedSourceRevision: sourceRevision,
	})
	if err != nil {
		return err
	}
	if !hasOverride(overrides, "album_artist_name") && updated.AlbumID.Valid && len(albumArtists) > 0 {
		album, albumErr := q.GetMusicAlbum(ctx, repo.GetMusicAlbumParams{AlbumID: updated.AlbumID.UUID, OwnerID: ownerID})
		if albumErr != nil {
			return albumErr
		}
		albumArtistsManual = album.ArtistSource == "manual"
	}
	if !hasOverride(overrides, "artist_name") {
		if err := replaceTrackArtistsTx(ctx, q, ownerID, updated.TrackID, artists, artistIDs); err != nil {
			return err
		}
	}
	if !hasOverride(overrides, "album_artist_name") && !albumArtistsManual && updated.AlbumID.Valid && len(albumArtists) > 0 {
		if err := replaceAlbumArtistsTx(ctx, q, ownerID, updated.AlbumID.UUID, albumArtists, albumArtistIDs); err != nil {
			return err
		}
	}
	return nil
}

func (s *musicService) UpdateTrack(ctx context.Context, ownerID int32, trackID uuid.UUID, patch MusicTrackPatch, expectedRevision int64) (MusicTrack, error) {
	current, err := s.GetTrack(ctx, ownerID, trackID)
	if err != nil {
		return MusicTrack{}, err
	}
	if expectedRevision <= 0 {
		expectedRevision = current.Revision
	}
	updated := current
	if patch.Title != nil {
		updated.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.AlbumTitle != nil {
		updated.AlbumTitle = strings.TrimSpace(*patch.AlbumTitle)
	}
	if patch.ArtistName != nil {
		updated.ArtistName = strings.TrimSpace(*patch.ArtistName)
	}
	if patch.AlbumArtistName != nil {
		updated.AlbumArtistName = strings.TrimSpace(*patch.AlbumArtistName)
	}
	if patch.Genre != nil {
		updated.Genre = strings.TrimSpace(*patch.Genre)
	}
	if patch.ReleaseDate != nil {
		updated.ReleaseDate = optionalString(strings.TrimSpace(*patch.ReleaseDate))
	}
	if patch.ReleasePrecision != nil {
		updated.ReleasePrecision = strings.TrimSpace(*patch.ReleasePrecision)
		if _, err := validateMusicReleasePrecision(updated.ReleasePrecision); err != nil {
			return MusicTrack{}, err
		}
	}
	if patch.Edition != nil {
		updated.Edition = strings.TrimSpace(*patch.Edition)
	}
	if patch.DiscNumber != nil {
		updated.DiscNumber = positiveOrNil(*patch.DiscNumber)
	}
	if patch.DiscTotal != nil {
		updated.DiscTotal = positiveOrNil(*patch.DiscTotal)
	}
	if patch.TrackNumber != nil {
		updated.TrackNumber = positiveOrNil(*patch.TrackNumber)
	}
	if patch.TrackTotal != nil {
		updated.TrackTotal = positiveOrNil(*patch.TrackTotal)
	}
	if patch.Compilation != nil {
		updated.Compilation = *patch.Compilation
	}
	if patch.Designation != nil {
		if *patch.Designation != "music" && *patch.Designation != "other" {
			return MusicTrack{}, fmt.Errorf("%w: designation must be music or other", ErrMusicInvalid)
		}
		updated.Designation = *patch.Designation
	}
	if patch.AlbumID != nil {
		if *patch.AlbumID != uuid.Nil {
			if _, albumErr := s.readQueries.GetMusicAlbum(ctx, repo.GetMusicAlbumParams{AlbumID: *patch.AlbumID, OwnerID: ownerID}); albumErr != nil {
				return MusicTrack{}, albumErr
			}
		}
		updated.AlbumID = patch.AlbumID
	}
	artistNames := []string(nil)
	if patch.Artists != nil {
		artistNames = normalizeMusicCredits(*patch.Artists)
		updated.ArtistName = firstString(artistNames)
	}
	albumArtistNames := []string(nil)
	if patch.AlbumArtists != nil {
		albumArtistNames = normalizeMusicCredits(*patch.AlbumArtists)
		updated.AlbumArtistName = firstString(albumArtistNames)
	}
	values := musicTrackValues{
		Designation: updated.Designation, AlbumID: uuidNull(updated.AlbumID), Title: updated.Title,
		AlbumTitle: updated.AlbumTitle, ArtistName: updated.ArtistName, AlbumArtistName: updated.AlbumArtistName,
		Genre: updated.Genre, ReleaseDate: updated.ReleaseDate, ReleasePrecision: updated.ReleasePrecision,
		Edition: updated.Edition, DiscNumber: updated.DiscNumber, DiscTotal: updated.DiscTotal,
		TrackNumber: updated.TrackNumber, TrackTotal: updated.TrackTotal, Compilation: boolInt(updated.Compilation),
	}
	fields := map[string]*string{
		"title": patch.Title, "album_title": patch.AlbumTitle, "artist_name": patch.ArtistName,
		"album_artist_name": patch.AlbumArtistName, "genre": patch.Genre, "release_date": patch.ReleaseDate,
		"release_precision": patch.ReleasePrecision, "edition": patch.Edition, "designation": patch.Designation,
	}
	if patch.Artists != nil && patch.ArtistName == nil {
		artistName := updated.ArtistName
		fields["artist_name"] = &artistName
	}
	if patch.AlbumArtists != nil && patch.AlbumArtistName == nil {
		albumArtistName := updated.AlbumArtistName
		fields["album_artist_name"] = &albumArtistName
	}
	if patch.AlbumID != nil {
		albumValue := ""
		if *patch.AlbumID != uuid.Nil {
			albumValue = patch.AlbumID.String()
		}
		fields["album_id"] = &albumValue
	}
	numericFields := map[string]*int64{
		"disc_number": patch.DiscNumber, "disc_total": patch.DiscTotal,
		"track_number": patch.TrackNumber, "track_total": patch.TrackTotal,
	}
	returnValue := MusicTrack{}
	err = s.write(ctx, func(tx *sql.Tx, q *repo.Queries) error {
		_, updateErr := q.UpdateMusicTrack(ctx, repo.UpdateMusicTrackParams{
			Designation: values.Designation, AlbumID: values.AlbumID, Title: values.Title,
			AlbumTitle: values.AlbumTitle, ArtistName: values.ArtistName, AlbumArtistName: values.AlbumArtistName,
			Genre: values.Genre, ReleaseDate: values.ReleaseDate, ReleasePrecision: values.ReleasePrecision,
			Edition: values.Edition, DiscNumber: values.DiscNumber, DiscTotal: values.DiscTotal,
			TrackNumber: values.TrackNumber, TrackTotal: values.TrackTotal, IsCompilation: values.Compilation,
			TrackID: trackID, OwnerID: ownerID, ExpectedRevision: expectedRevision,
		})
		if updateErr != nil {
			if errors.Is(updateErr, sql.ErrNoRows) {
				return ErrMusicConflict
			}
			return updateErr
		}
		for field, value := range fields {
			if value == nil {
				continue
			}
			textValue := strings.TrimSpace(*value)
			present := int64(1)
			if field == "album_id" {
				if *value == "" {
					present = 0
				}
			}
			var stored *string
			if textValue == "" {
				present = 0
			} else {
				stored = &textValue
			}
			if err := q.UpsertMusicTrackOverride(ctx, repo.UpsertMusicTrackOverrideParams{TrackID: trackID.String(), Field: field, Value: stored, IsPresent: present}); err != nil {
				return err
			}
		}
		for field, value := range numericFields {
			if value == nil {
				continue
			}
			textValue := strconv.FormatInt(*value, 10)
			if *value <= 0 {
				textValue = ""
			}
			present := int64(1)
			var stored *string
			if textValue == "" {
				present = 0
			} else {
				stored = &textValue
			}
			if err := q.UpsertMusicTrackOverride(ctx, repo.UpsertMusicTrackOverrideParams{TrackID: trackID.String(), Field: field, Value: stored, IsPresent: present}); err != nil {
				return err
			}
		}
		if patch.Compilation != nil {
			value := "0"
			if *patch.Compilation {
				value = "1"
			}
			if err := q.UpsertMusicTrackOverride(ctx, repo.UpsertMusicTrackOverrideParams{
				TrackID: trackID.String(), Field: "compilation", Value: &value, IsPresent: 1,
			}); err != nil {
				return err
			}
		}
		if patch.ArtistName != nil {
			if err := replaceTrackArtistsTx(ctx, q, ownerID, trackID, []string{updated.ArtistName}, nil); err != nil {
				return err
			}
		}
		if patch.Artists != nil {
			if err := replaceTrackArtistsTx(ctx, q, ownerID, trackID, artistNames, nil); err != nil {
				return err
			}
		}
		if patch.AlbumArtistName != nil && updated.AlbumID != nil && *updated.AlbumID != uuid.Nil {
			if err := replaceAlbumArtistsTx(ctx, q, ownerID, *updated.AlbumID, []string{updated.AlbumArtistName}, nil); err != nil {
				return err
			}
		}
		if patch.AlbumArtists != nil && updated.AlbumID != nil && *updated.AlbumID != uuid.Nil {
			if err := replaceAlbumArtistsTx(ctx, q, ownerID, *updated.AlbumID, albumArtistNames, nil); err != nil {
				return err
			}
		}
		if (patch.AlbumArtistName != nil || patch.AlbumArtists != nil) && updated.AlbumID != nil && *updated.AlbumID != uuid.Nil {
			if err := q.SetMusicAlbumArtistSource(ctx, repo.SetMusicAlbumArtistSourceParams{
				AlbumID: *updated.AlbumID, OwnerID: ownerID, ArtistSource: "manual",
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return MusicTrack{}, err
	}
	returnValue, err = s.GetTrack(ctx, ownerID, trackID)
	return returnValue, err
}

func positiveOrNil(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func (s *musicService) ResetTrackOverrides(ctx context.Context, ownerID int32, trackID uuid.UUID) (MusicTrack, error) {
	_, err := s.GetTrack(ctx, ownerID, trackID)
	if err != nil {
		return MusicTrack{}, err
	}
	err = s.write(ctx, func(tx *sql.Tx, q *repo.Queries) error {
		if err := q.DeleteMusicTrackOverrides(ctx, trackID.String()); err != nil {
			return err
		}
		asset, err := q.GetAssetByIDAny(ctx, trackID)
		if err != nil {
			return err
		}
		return SyncMusicTrackFromAssetTx(ctx, tx, q, trackID, asset.SpecificMetadata)
	})
	if err != nil {
		return MusicTrack{}, err
	}
	return s.GetTrack(ctx, ownerID, trackID)
}

func (s *musicService) SetTrackDesignation(ctx context.Context, ownerID int32, trackID uuid.UUID, designation string, expectedRevision int64) (MusicTrack, error) {
	return s.UpdateTrack(ctx, ownerID, trackID, MusicTrackPatch{Designation: &designation}, expectedRevision)
}

func validateMusicReleasePrecision(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown", nil
	}
	switch value {
	case "unknown", "year", "month", "day":
		return value, nil
	default:
		return "", fmt.Errorf("%w: release precision must be unknown, year, month, or day", ErrMusicInvalid)
	}
}

func albumFromListRow(row repo.ListMusicAlbumsRow) MusicAlbum {
	return MusicAlbum{
		AlbumID: row.AlbumID, OwnerID: row.OwnerID, Title: row.Title,
		ReleaseDate: row.ReleaseDate, ReleasePrecision: row.ReleasePrecision,
		Edition: row.Edition, ReleaseIdentifier: row.ReleaseIdentifier,
		CoverAssetID: nullUUIDPointer(row.CoverAssetID), Revision: row.Revision,
		Favorite: row.Favorite != 0, TrackCount: row.TrackCount, artistSource: row.ArtistSource,
	}
}

func albumFromDetailRow(row repo.GetMusicAlbumRow) MusicAlbum {
	return MusicAlbum{
		AlbumID: row.AlbumID, OwnerID: row.OwnerID, Title: row.Title,
		ReleaseDate: row.ReleaseDate, ReleasePrecision: row.ReleasePrecision,
		Edition: row.Edition, ReleaseIdentifier: row.ReleaseIdentifier,
		CoverAssetID: nullUUIDPointer(row.CoverAssetID), Revision: row.Revision,
		Favorite: row.Favorite != 0, TrackCount: row.TrackCount, artistSource: row.ArtistSource,
	}
}

func trackFromAlbumTrackRow(row repo.ListMusicAlbumTracksRow) MusicTrack {
	return MusicTrack{
		TrackID: row.TrackID, OwnerID: row.OwnerID, Designation: row.Designation,
		AlbumID: nullUUIDPointer(row.AlbumID), Title: row.Title, AlbumTitle: row.AlbumTitle,
		ArtistName: row.ArtistName, AlbumArtistName: row.AlbumArtistName, Genre: row.Genre,
		ReleaseDate: row.ReleaseDate, ReleasePrecision: row.ReleasePrecision, Edition: row.Edition,
		ReleaseIdentifier: row.ReleaseIdentifier, DiscNumber: row.DiscNumber, DiscTotal: row.DiscTotal,
		TrackNumber: row.TrackNumber, TrackTotal: row.TrackTotal, Compilation: row.IsCompilation != 0,
		ExtractedSourceRevision: row.ExtractedSourceRevision, Revision: row.Revision,
		OriginalFilename: row.OriginalFilename, MimeType: row.MimeType, Duration: row.Duration,
		TakenAt: timestampTime(row.TakenTime), IsDeleted: row.IsDeleted, Liked: row.Liked,
	}
}

func (s *musicService) ListAlbums(ctx context.Context, ownerID int32, query string, favoritesOnly bool, limit, offset int) (MusicAlbumPage, error) {
	limit, offset = clampMusicPage(limit, offset)
	query = strings.TrimSpace(query)
	rows, err := s.readQueries.ListMusicAlbums(ctx, repo.ListMusicAlbumsParams{
		OwnerID: ownerID, FavoritesOnly: boolInt(favoritesOnly), Query: query, Limit: int64(limit), Offset: int64(offset),
	})
	if err != nil {
		return MusicAlbumPage{}, err
	}
	total, err := s.readQueries.CountMusicAlbums(ctx, repo.CountMusicAlbumsParams{OwnerID: ownerID, FavoritesOnly: boolInt(favoritesOnly), Query: query})
	if err != nil {
		return MusicAlbumPage{}, err
	}
	items := make([]MusicAlbum, 0, len(rows))
	for _, row := range rows {
		album := albumFromListRow(row)
		credits, err := s.readQueries.ListMusicAlbumArtists(ctx, repo.ListMusicAlbumArtistsParams{AlbumID: row.AlbumID, OwnerID: ownerID})
		if err != nil {
			return MusicAlbumPage{}, err
		}
		album.Artists = creditsFromAlbumRows(credits)
		if err := s.resolveAlbumCover(ctx, ownerID, &album); err != nil {
			return MusicAlbumPage{}, err
		}
		items = append(items, album)
	}
	return MusicAlbumPage{Items: items, Total: total, Offset: offset, Limit: limit}, nil
}

func (s *musicService) GetAlbum(ctx context.Context, ownerID int32, albumID uuid.UUID) (MusicAlbum, error) {
	row, err := s.readQueries.GetMusicAlbum(ctx, repo.GetMusicAlbumParams{AlbumID: albumID, OwnerID: ownerID})
	if err != nil {
		return MusicAlbum{}, err
	}
	album := albumFromDetailRow(row)
	if err := s.resolveAlbumCover(ctx, ownerID, &album); err != nil {
		return MusicAlbum{}, err
	}
	artistRows, err := s.readQueries.ListMusicAlbumArtists(ctx, repo.ListMusicAlbumArtistsParams{AlbumID: albumID, OwnerID: ownerID})
	if err != nil {
		return MusicAlbum{}, err
	}
	album.Artists = creditsFromAlbumRows(artistRows)
	trackRows, err := s.readQueries.ListMusicAlbumTracks(ctx, repo.ListMusicAlbumTracksParams{AlbumID: uuidNull(&albumID), OwnerID: ownerID})
	if err != nil {
		return MusicAlbum{}, err
	}
	album.Tracks = make([]MusicTrack, 0, len(trackRows))
	for _, trackRow := range trackRows {
		track, enrichErr := s.enrichTrack(ctx, ownerID, trackFromAlbumTrackRow(trackRow))
		if enrichErr != nil {
			return MusicAlbum{}, enrichErr
		}
		album.Tracks = append(album.Tracks, track)
	}
	return album, nil
}

func (s *musicService) validateCoverAsset(ctx context.Context, ownerID int32, assetID *uuid.UUID) error {
	if assetID == nil || *assetID == uuid.Nil {
		return nil
	}
	asset, err := s.readQueries.GetAssetByIDAny(ctx, *assetID)
	if err != nil {
		return err
	}
	if asset.OwnerID == nil || *asset.OwnerID != ownerID || asset.IsDeleted {
		return fmt.Errorf("%w: cover asset is not owned by the current user", ErrMusicInvalid)
	}
	return nil
}

func (s *musicService) CreateAlbum(ctx context.Context, ownerID int32, title string, releaseDate *string, releasePrecision, edition string, coverAssetID *uuid.UUID, artistNames []string) (MusicAlbum, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return MusicAlbum{}, fmt.Errorf("%w: album title is required", ErrMusicInvalid)
	}
	precision, err := validateMusicReleasePrecision(releasePrecision)
	if err != nil {
		return MusicAlbum{}, err
	}
	if err := s.validateCoverAsset(ctx, ownerID, coverAssetID); err != nil {
		return MusicAlbum{}, err
	}
	if releaseDate != nil {
		value := strings.TrimSpace(*releaseDate)
		releaseDate = optionalString(value)
	}
	var albumID uuid.UUID
	err = s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		created, createErr := q.CreateMusicAlbum(ctx, repo.CreateMusicAlbumParams{
			AlbumID: uuid.New(), OwnerID: ownerID, Title: title, ReleaseDate: releaseDate,
			ReleasePrecision: precision, Edition: strings.TrimSpace(edition), ReleaseIdentifier: "",
			SourceGroup: "", ArtistSource: "manual", CoverAssetID: uuidNull(coverAssetID),
		})
		if createErr != nil {
			return createErr
		}
		albumID = created.AlbumID
		return replaceAlbumArtistsTx(ctx, q, ownerID, albumID, artistNames, nil)
	})
	if err != nil {
		return MusicAlbum{}, err
	}
	return s.GetAlbum(ctx, ownerID, albumID)
}

func (s *musicService) UpdateAlbum(ctx context.Context, ownerID int32, albumID uuid.UUID, patch MusicAlbumPatch, expectedRevision int64) (MusicAlbum, error) {
	row, err := s.readQueries.GetMusicAlbum(ctx, repo.GetMusicAlbumParams{AlbumID: albumID, OwnerID: ownerID})
	if err != nil {
		return MusicAlbum{}, err
	}
	// Persist only explicit artwork; a read-time embedded cover must stay derived.
	current := albumFromDetailRow(row)
	if expectedRevision <= 0 {
		expectedRevision = current.Revision
	}
	updated := current
	if patch.Favorite != nil {
		updated.Favorite = *patch.Favorite
	}
	if patch.Title != nil {
		updated.Title = strings.TrimSpace(*patch.Title)
	}
	if updated.Title == "" {
		return MusicAlbum{}, fmt.Errorf("%w: album title is required", ErrMusicInvalid)
	}
	if patch.ReleaseDate != nil {
		updated.ReleaseDate = optionalString(strings.TrimSpace(*patch.ReleaseDate))
	}
	if patch.ReleasePrecision != nil {
		updated.ReleasePrecision, err = validateMusicReleasePrecision(*patch.ReleasePrecision)
		if err != nil {
			return MusicAlbum{}, err
		}
	}
	if patch.Edition != nil {
		updated.Edition = strings.TrimSpace(*patch.Edition)
	}
	if patch.CoverAssetID != nil {
		if err := s.validateCoverAsset(ctx, ownerID, patch.CoverAssetID); err != nil {
			return MusicAlbum{}, err
		}
		updated.CoverAssetID = patch.CoverAssetID
	}
	artistNames := []string(nil)
	if patch.ArtistNames != nil {
		artistNames = normalizeMusicCredits(*patch.ArtistNames)
	}
	artistSource := current.artistSource
	if artistSource == "" {
		artistSource = "extracted"
	}
	if patch.ArtistNames != nil {
		artistSource = "manual"
	}
	err = s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		_, updateErr := q.UpdateMusicAlbum(ctx, repo.UpdateMusicAlbumParams{
			Favorite: boolInt(updated.Favorite), Title: updated.Title, ReleaseDate: updated.ReleaseDate, ReleasePrecision: updated.ReleasePrecision,
			Edition: updated.Edition, ArtistSource: artistSource, CoverAssetID: uuidNull(updated.CoverAssetID), AlbumID: albumID,
			OwnerID: ownerID, ExpectedRevision: expectedRevision,
		})
		if errors.Is(updateErr, sql.ErrNoRows) {
			return ErrMusicConflict
		}
		if updateErr != nil {
			return updateErr
		}
		if patch.ArtistNames != nil {
			return replaceAlbumArtistsTx(ctx, q, ownerID, albumID, artistNames, nil)
		}
		return nil
	})
	if err != nil {
		return MusicAlbum{}, err
	}
	return s.GetAlbum(ctx, ownerID, albumID)
}

func (s *musicService) AssignTrackAlbum(ctx context.Context, ownerID int32, trackID uuid.UUID, albumID *uuid.UUID, expectedRevision int64) error {
	if albumID == nil {
		cleared := uuid.Nil
		albumID = &cleared
	}
	_, err := s.UpdateTrack(ctx, ownerID, trackID, MusicTrackPatch{AlbumID: albumID}, expectedRevision)
	return err
}

func artistFromListRow(row repo.ListMusicArtistsRow) MusicArtist {
	return MusicArtist{
		ArtistID: row.ArtistID, OwnerID: row.OwnerID, DisplayName: row.DisplayName,
		Favorite: row.Favorite != 0, Revision: row.Revision, TrackCount: row.TrackCount, AlbumCount: row.AlbumCount,
	}
}

func artistFromDetailRow(row repo.GetMusicArtistRow) MusicArtist {
	return MusicArtist{
		ArtistID: row.ArtistID, OwnerID: row.OwnerID, DisplayName: row.DisplayName,
		Favorite: row.Favorite != 0, Revision: row.Revision, TrackCount: row.TrackCount, AlbumCount: row.AlbumCount,
	}
}

func (s *musicService) ListArtists(ctx context.Context, ownerID int32, query string, favoritesOnly bool, limit, offset int) (MusicArtistPage, error) {
	limit, offset = clampMusicPage(limit, offset)
	query = strings.TrimSpace(query)
	rows, err := s.readQueries.ListMusicArtists(ctx, repo.ListMusicArtistsParams{
		OwnerID: ownerID, FavoritesOnly: boolInt(favoritesOnly), Query: query, Limit: int64(limit), Offset: int64(offset),
	})
	if err != nil {
		return MusicArtistPage{}, err
	}
	total, err := s.readQueries.CountMusicArtists(ctx, repo.CountMusicArtistsParams{OwnerID: ownerID, FavoritesOnly: boolInt(favoritesOnly), Query: query})
	if err != nil {
		return MusicArtistPage{}, err
	}
	items := make([]MusicArtist, 0, len(rows))
	for _, row := range rows {
		items = append(items, artistFromListRow(row))
	}
	return MusicArtistPage{Items: items, Total: total, Offset: offset, Limit: limit}, nil
}

func (s *musicService) GetArtist(ctx context.Context, ownerID int32, artistID uuid.UUID) (MusicArtist, error) {
	row, err := s.readQueries.GetMusicArtist(ctx, repo.GetMusicArtistParams{ArtistID: artistID, OwnerID: ownerID})
	if err != nil {
		return MusicArtist{}, err
	}
	return artistFromDetailRow(row), nil
}

func (s *musicService) UpdateArtist(ctx context.Context, ownerID int32, artistID uuid.UUID, patch MusicArtistPatch, expectedRevision int64) (MusicArtist, error) {
	current, err := s.GetArtist(ctx, ownerID, artistID)
	if err != nil {
		return MusicArtist{}, err
	}
	if expectedRevision <= 0 {
		expectedRevision = current.Revision
	}
	favorite := current.Favorite
	if patch.Favorite != nil {
		favorite = *patch.Favorite
	}
	name := current.DisplayName
	if patch.DisplayName != nil {
		name = strings.TrimSpace(*patch.DisplayName)
	}
	if name == "" {
		return MusicArtist{}, fmt.Errorf("%w: artist name is required", ErrMusicInvalid)
	}
	err = s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		_, updateErr := q.UpdateMusicArtist(ctx, repo.UpdateMusicArtistParams{
			Favorite: boolInt(favorite), DisplayName: name, NormalizedName: normalizeMusicName(name), ArtistID: artistID,
			OwnerID: ownerID, ExpectedRevision: expectedRevision,
		})
		if errors.Is(updateErr, sql.ErrNoRows) {
			return ErrMusicConflict
		}
		return updateErr
	})
	if err != nil {
		return MusicArtist{}, err
	}
	return s.GetArtist(ctx, ownerID, artistID)
}

func playlistFromListRow(row repo.ListMusicPlaylistsRow) MusicPlaylist {
	return MusicPlaylist{
		PlaylistID: row.PlaylistID, OwnerID: row.OwnerID, Title: row.Title,
		Description: row.Description, Revision: row.Revision, EntryCount: row.EntryCount,
	}
}

func playlistFromDetailRow(row repo.GetMusicPlaylistRow) MusicPlaylist {
	return MusicPlaylist{
		PlaylistID: row.PlaylistID, OwnerID: row.OwnerID, Title: row.Title,
		Description: row.Description, Revision: row.Revision, EntryCount: row.EntryCount,
	}
}

func playlistEntryFromRow(row repo.ListMusicPlaylistEntriesRow) MusicPlaylistEntry {
	entry := MusicPlaylistEntry{
		EntryID: row.EntryID, PlaylistID: row.PlaylistID, TrackID: nullUUIDPointer(row.TrackID),
		SavedTitle: row.SavedTitle, Position: row.Position, IdempotencyKey: row.IdempotencyKey,
	}
	entry.Available = row.TrackID.Valid && row.TrackTitle != nil && !row.IsDeleted
	return entry
}

func (s *musicService) ListPlaylists(ctx context.Context, ownerID int32, limit, offset int) (MusicPlaylistPage, error) {
	limit, offset = clampMusicPage(limit, offset)
	rows, err := s.readQueries.ListMusicPlaylists(ctx, repo.ListMusicPlaylistsParams{
		OwnerID: ownerID, Limit: int64(limit), Offset: int64(offset),
	})
	if err != nil {
		return MusicPlaylistPage{}, err
	}
	total, err := s.readQueries.CountMusicPlaylists(ctx, ownerID)
	if err != nil {
		return MusicPlaylistPage{}, err
	}
	items := make([]MusicPlaylist, 0, len(rows))
	for _, row := range rows {
		items = append(items, playlistFromListRow(row))
	}
	return MusicPlaylistPage{Items: items, Total: total, Offset: offset, Limit: limit}, nil
}

func (s *musicService) GetPlaylist(ctx context.Context, ownerID int32, playlistID uuid.UUID) (MusicPlaylist, error) {
	row, err := s.readQueries.GetMusicPlaylist(ctx, repo.GetMusicPlaylistParams{PlaylistID: playlistID, OwnerID: ownerID})
	if err != nil {
		return MusicPlaylist{}, err
	}
	return playlistFromDetailRow(row), nil
}

func (s *musicService) CreatePlaylist(ctx context.Context, ownerID int32, title, description string) (MusicPlaylist, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return MusicPlaylist{}, fmt.Errorf("%w: playlist title is required", ErrMusicInvalid)
	}
	var playlistID uuid.UUID
	err := s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		created, createErr := q.CreateMusicPlaylist(ctx, repo.CreateMusicPlaylistParams{
			PlaylistID: uuid.New(), OwnerID: ownerID, Title: title, Description: strings.TrimSpace(description),
		})
		if createErr != nil {
			return createErr
		}
		playlistID = created.PlaylistID
		return nil
	})
	if err != nil {
		return MusicPlaylist{}, err
	}
	return s.GetPlaylist(ctx, ownerID, playlistID)
}

func (s *musicService) UpdatePlaylist(ctx context.Context, ownerID int32, playlistID uuid.UUID, title, description string, expectedRevision int64) (MusicPlaylist, error) {
	current, err := s.GetPlaylist(ctx, ownerID, playlistID)
	if err != nil {
		return MusicPlaylist{}, err
	}
	if expectedRevision <= 0 {
		expectedRevision = current.Revision
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return MusicPlaylist{}, fmt.Errorf("%w: playlist title is required", ErrMusicInvalid)
	}
	err = s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		_, updateErr := q.UpdateMusicPlaylist(ctx, repo.UpdateMusicPlaylistParams{
			Title: title, Description: strings.TrimSpace(description), PlaylistID: playlistID,
			OwnerID: ownerID, ExpectedRevision: expectedRevision,
		})
		if errors.Is(updateErr, sql.ErrNoRows) {
			return ErrMusicConflict
		}
		return updateErr
	})
	if err != nil {
		return MusicPlaylist{}, err
	}
	return s.GetPlaylist(ctx, ownerID, playlistID)
}

func (s *musicService) DeletePlaylist(ctx context.Context, ownerID int32, playlistID uuid.UUID) error {
	if _, err := s.GetPlaylist(ctx, ownerID, playlistID); err != nil {
		return err
	}
	return s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		return q.DeleteMusicPlaylist(ctx, repo.DeleteMusicPlaylistParams{PlaylistID: playlistID, OwnerID: ownerID})
	})
}

func (s *musicService) ListPlaylistEntries(ctx context.Context, ownerID int32, playlistID uuid.UUID) ([]MusicPlaylistEntry, error) {
	if _, err := s.GetPlaylist(ctx, ownerID, playlistID); err != nil {
		return nil, err
	}
	rows, err := s.readQueries.ListMusicPlaylistEntries(ctx, repo.ListMusicPlaylistEntriesParams{PlaylistID: playlistID, OwnerID: ownerID})
	if err != nil {
		return nil, err
	}
	entries := make([]MusicPlaylistEntry, 0, len(rows))
	for _, row := range rows {
		entry := playlistEntryFromRow(row)
		if entry.Available && entry.TrackID != nil {
			track, trackErr := s.GetTrack(ctx, ownerID, *entry.TrackID)
			if trackErr == nil {
				entry.Track = &track
			} else if errors.Is(trackErr, sql.ErrNoRows) || errors.Is(trackErr, ErrMusicNotFound) {
				entry.Available = false
			} else {
				return nil, trackErr
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func findPlaylistEntry(entries []MusicPlaylistEntry, entryID string) (MusicPlaylistEntry, error) {
	for _, entry := range entries {
		if entry.EntryID == entryID {
			return entry, nil
		}
	}
	return MusicPlaylistEntry{}, ErrMusicNotFound
}

func normalizeOptionalKey(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *musicService) AddPlaylistEntry(ctx context.Context, ownerID int32, playlistID, trackID uuid.UUID, idempotencyKey string, savedTitle *string, expectedRevision int64) (MusicPlaylistEntry, error) {
	playlist, err := s.GetPlaylist(ctx, ownerID, playlistID)
	if err != nil {
		return MusicPlaylistEntry{}, err
	}
	if _, err := s.GetTrack(ctx, ownerID, trackID); err != nil {
		return MusicPlaylistEntry{}, err
	}
	if expectedRevision <= 0 {
		expectedRevision = playlist.Revision
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	var savedEntryID string
	err = s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		if idempotencyKey != "" {
			key := idempotencyKey
			existing, lookupErr := q.GetMusicPlaylistEntryByIdempotency(ctx, repo.GetMusicPlaylistEntryByIdempotencyParams{
				PlaylistID: playlistID, IdempotencyKey: &key, OwnerID: ownerID,
			})
			if lookupErr == nil {
				savedEntryID = existing.EntryID
				return nil
			}
			if !errors.Is(lookupErr, sql.ErrNoRows) {
				return lookupErr
			}
		}
		current, getErr := q.GetMusicPlaylist(ctx, repo.GetMusicPlaylistParams{PlaylistID: playlistID, OwnerID: ownerID})
		if getErr != nil {
			return getErr
		}
		if current.Revision != expectedRevision {
			return ErrMusicConflict
		}
		if _, getErr := q.GetMusicTrack(ctx, repo.GetMusicTrackParams{TrackID: trackID, OwnerID: ownerID}); getErr != nil {
			return getErr
		}
		entries, listErr := q.ListMusicPlaylistEntries(ctx, repo.ListMusicPlaylistEntriesParams{PlaylistID: playlistID, OwnerID: ownerID})
		if listErr != nil {
			return listErr
		}
		position := int64(0)
		for _, entry := range entries {
			if entry.Position >= position {
				position = entry.Position + 1
			}
		}
		var saved interface{}
		if savedTitle != nil {
			saved = strings.TrimSpace(*savedTitle)
		}
		entryID := uuid.New().String()
		if err := q.CreateMusicPlaylistEntry(ctx, repo.CreateMusicPlaylistEntryParams{
			EntryID: entryID, PlaylistID: playlistID, OwnerID: ownerID, TrackID: trackID,
			SavedTitle: saved, Position: position, IdempotencyKey: normalizeOptionalKey(&idempotencyKey),
		}); err != nil {
			return err
		}
		savedEntryID = entryID
		return bumpMusicPlaylistRevisionTx(ctx, q, playlistID, ownerID, expectedRevision)
	})
	if err != nil {
		return MusicPlaylistEntry{}, err
	}
	entries, err := s.ListPlaylistEntries(ctx, ownerID, playlistID)
	if err != nil {
		return MusicPlaylistEntry{}, err
	}
	return findPlaylistEntry(entries, savedEntryID)
}

func bumpMusicPlaylistRevisionTx(ctx context.Context, q *repo.Queries, playlistID uuid.UUID, ownerID int32, expectedRevision int64) error {
	if err := q.BumpMusicPlaylistRevision(ctx, repo.BumpMusicPlaylistRevisionParams{
		PlaylistID: playlistID, OwnerID: ownerID, ExpectedRevision: expectedRevision,
	}); err != nil {
		return err
	}
	updated, err := q.GetMusicPlaylist(ctx, repo.GetMusicPlaylistParams{PlaylistID: playlistID, OwnerID: ownerID})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMusicConflict
	}
	if err != nil {
		return err
	}
	if updated.Revision <= expectedRevision {
		return ErrMusicConflict
	}
	return nil
}

func (s *musicService) RemovePlaylistEntry(ctx context.Context, ownerID int32, playlistID uuid.UUID, entryID string, expectedRevision int64) error {
	playlist, err := s.GetPlaylist(ctx, ownerID, playlistID)
	if err != nil {
		return err
	}
	if expectedRevision <= 0 {
		expectedRevision = playlist.Revision
	}
	return s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		current, getErr := q.GetMusicPlaylist(ctx, repo.GetMusicPlaylistParams{PlaylistID: playlistID, OwnerID: ownerID})
		if getErr != nil {
			return getErr
		}
		if current.Revision != expectedRevision {
			return ErrMusicConflict
		}
		entries, listErr := q.ListMusicPlaylistEntries(ctx, repo.ListMusicPlaylistEntriesParams{PlaylistID: playlistID, OwnerID: ownerID})
		if listErr != nil {
			return listErr
		}
		if _, findErr := findRepoPlaylistEntry(entries, entryID); findErr != nil {
			return findErr
		}
		if err := q.DeleteMusicPlaylistEntry(ctx, repo.DeleteMusicPlaylistEntryParams{EntryID: entryID, PlaylistID: playlistID, OwnerID: ownerID}); err != nil {
			return err
		}
		position := int64(0)
		for _, entry := range entries {
			if entry.EntryID == entryID {
				continue
			}
			if err := q.UpdateMusicPlaylistEntryPosition(ctx, repo.UpdateMusicPlaylistEntryPositionParams{
				Position: position, EntryID: entry.EntryID, PlaylistID: playlistID, OwnerID: ownerID,
			}); err != nil {
				return err
			}
			position++
		}
		return bumpMusicPlaylistRevisionTx(ctx, q, playlistID, ownerID, expectedRevision)
	})
}

func findRepoPlaylistEntry(entries []repo.ListMusicPlaylistEntriesRow, entryID string) (repo.ListMusicPlaylistEntriesRow, error) {
	for _, entry := range entries {
		if entry.EntryID == entryID {
			return entry, nil
		}
	}
	return repo.ListMusicPlaylistEntriesRow{}, ErrMusicNotFound
}

func (s *musicService) ReorderPlaylist(ctx context.Context, ownerID int32, playlistID uuid.UUID, positions []MusicEntryPosition, expectedRevision int64) error {
	playlist, err := s.GetPlaylist(ctx, ownerID, playlistID)
	if err != nil {
		return err
	}
	if expectedRevision <= 0 {
		expectedRevision = playlist.Revision
	}
	return s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		current, getErr := q.GetMusicPlaylist(ctx, repo.GetMusicPlaylistParams{PlaylistID: playlistID, OwnerID: ownerID})
		if getErr != nil {
			return getErr
		}
		if current.Revision != expectedRevision {
			return ErrMusicConflict
		}
		entries, listErr := q.ListMusicPlaylistEntries(ctx, repo.ListMusicPlaylistEntriesParams{PlaylistID: playlistID, OwnerID: ownerID})
		if listErr != nil {
			return listErr
		}
		if len(entries) != len(positions) {
			return fmt.Errorf("%w: reorder must contain every playlist entry", ErrMusicInvalid)
		}
		expectedIDs := make(map[string]struct{}, len(entries))
		for _, entry := range entries {
			expectedIDs[entry.EntryID] = struct{}{}
		}
		seenPositions := make(map[int64]struct{}, len(positions))
		for _, requested := range positions {
			if _, ok := expectedIDs[requested.EntryID]; !ok {
				return fmt.Errorf("%w: reorder contains an unknown entry", ErrMusicInvalid)
			}
			if requested.Position < 0 || requested.Position >= int64(len(entries)) {
				return fmt.Errorf("%w: reorder position is out of range", ErrMusicInvalid)
			}
			if _, ok := seenPositions[requested.Position]; ok {
				return fmt.Errorf("%w: reorder positions must be unique", ErrMusicInvalid)
			}
			seenPositions[requested.Position] = struct{}{}
		}
		if len(seenPositions) != len(entries) {
			return fmt.Errorf("%w: reorder positions must be contiguous", ErrMusicInvalid)
		}
		seenIDs := make(map[string]struct{}, len(positions))
		for _, requested := range positions {
			if _, ok := seenIDs[requested.EntryID]; ok {
				return fmt.Errorf("%w: reorder entries must be unique", ErrMusicInvalid)
			}
			seenIDs[requested.EntryID] = struct{}{}
			if err := q.UpdateMusicPlaylistEntryPosition(ctx, repo.UpdateMusicPlaylistEntryPositionParams{
				Position: requested.Position, EntryID: requested.EntryID, PlaylistID: playlistID, OwnerID: ownerID,
			}); err != nil {
				return err
			}
		}
		return bumpMusicPlaylistRevisionTx(ctx, q, playlistID, ownerID, expectedRevision)
	})
}

type playbackItem struct {
	trackID       *uuid.UUID
	sourceEntryID *string
	savedTitle    string
}

func (s *musicService) collectQueryPlaybackItems(ctx context.Context, ownerID int32, query, sort, artistFilter string, likedOnly bool) ([]playbackItem, int64, error) {
	items := make([]playbackItem, 0)
	offset := 0
	var total int64
	for {
		page, err := s.ListTracks(ctx, ownerID, query, sort, artistFilter, likedOnly, maxMusicPageSize, offset)
		if err != nil {
			return nil, 0, err
		}
		total = page.Total
		if total > maxPlaybackEntries {
			return nil, total, ErrPlaybackTooLarge
		}
		for index := range page.Items {
			trackID := page.Items[index].TrackID
			items = append(items, playbackItem{trackID: &trackID, savedTitle: page.Items[index].Title})
		}
		if len(page.Items) == 0 || len(items) >= int(total) {
			break
		}
		offset += len(page.Items)
	}
	return items, total, nil
}

func (s *musicService) collectPlaybackItems(ctx context.Context, ownerID int32, source PlaybackSource) ([]playbackItem, int64, int64, error) {
	source.Kind = strings.ToLower(strings.TrimSpace(source.Kind))
	var (
		items          []playbackItem
		total          int64
		sourceRevision int64
	)
	switch source.Kind {
	case "query", "liked":
		likedOnly := source.LikedOnly || source.Kind == "liked"
		var err error
		items, total, err = s.collectQueryPlaybackItems(ctx, ownerID, source.Query, source.Sort, source.ArtistID, likedOnly)
		if err != nil {
			return nil, 0, 0, err
		}
		sourceRevision = time.Now().UTC().UnixMicro()
	case "album":
		albumID, err := uuid.Parse(strings.TrimSpace(source.ID))
		if err != nil || albumID == uuid.Nil {
			return nil, 0, 0, fmt.Errorf("%w: album source id is invalid", ErrMusicInvalid)
		}
		album, err := s.GetAlbum(ctx, ownerID, albumID)
		if err != nil {
			return nil, 0, 0, err
		}
		sourceRevision = album.Revision
		items = make([]playbackItem, 0, len(album.Tracks))
		for index := range album.Tracks {
			track := album.Tracks[index]
			trackID := track.TrackID
			items = append(items, playbackItem{trackID: &trackID, savedTitle: track.Title})
		}
		total = int64(len(items))
	case "playlist":
		playlistID, err := uuid.Parse(strings.TrimSpace(source.ID))
		if err != nil || playlistID == uuid.Nil {
			return nil, 0, 0, fmt.Errorf("%w: playlist source id is invalid", ErrMusicInvalid)
		}
		playlist, err := s.GetPlaylist(ctx, ownerID, playlistID)
		if err != nil {
			return nil, 0, 0, err
		}
		entries, err := s.ListPlaylistEntries(ctx, ownerID, playlistID)
		if err != nil {
			return nil, 0, 0, err
		}
		sourceRevision = playlist.Revision
		items = make([]playbackItem, 0, len(entries))
		for index := range entries {
			entry := entries[index]
			var sourceEntryID = entry.EntryID
			items = append(items, playbackItem{
				trackID: entry.TrackID, sourceEntryID: &sourceEntryID, savedTitle: entry.SavedTitle,
			})
		}
		total = int64(len(items))
	default:
		return nil, 0, 0, fmt.Errorf("%w: playback source kind is invalid", ErrMusicInvalid)
	}
	if total > maxPlaybackEntries || len(items) > maxPlaybackEntries {
		return nil, total, sourceRevision, ErrPlaybackTooLarge
	}
	return items, total, sourceRevision, nil
}

func playbackSessionFromRow(row repo.MusicPlaybackSession) MusicPlaybackSession {
	return MusicPlaybackSession{
		SessionID: row.SessionID, OwnerID: row.OwnerID, SourceKind: row.SourceKind,
		SourceID: row.SourceID, SourceRevision: row.SourceRevision, Status: row.Status,
		ExpiresAt: row.ExpiresAt.Time, TotalEntries: 0,
	}
}

func playbackEntryFromRow(row repo.ListMusicPlaybackEntriesRow) MusicPlaybackEntry {
	title := stringOrEmpty(row.TrackTitle)
	return MusicPlaybackEntry{
		EntryID: row.EntryID, Sequence: row.Sequence, TrackID: nullUUIDPointer(row.TrackID),
		SourceEntryID: row.SourceEntryID, SavedTitle: row.SavedTitle,
		Available:  row.TrackID.Valid && title != "" && !row.IsDeleted,
		TrackTitle: title, TrackArtist: stringOrEmpty(row.TrackArtist), TrackAlbum: stringOrEmpty(row.TrackAlbum),
		MimeType: stringOrEmpty(row.MimeType), Duration: row.Duration,
	}
}

func (s *musicService) CreatePlaybackSession(ctx context.Context, ownerID int32, source PlaybackSource) (MusicPlaybackSession, error) {
	source.Kind = strings.ToLower(strings.TrimSpace(source.Kind))
	items, total, sourceRevision, err := s.collectPlaybackItems(ctx, ownerID, source)
	if err != nil {
		return MusicPlaybackSession{}, err
	}
	if total > maxPlaybackEntries {
		return MusicPlaybackSession{}, ErrPlaybackTooLarge
	}
	sourceID := strings.TrimSpace(source.ID)
	if source.Kind == "query" && sourceID == "" {
		sourceID = strings.TrimSpace(source.Query)
	}
	expiresAt := time.Now().UTC().Add(playbackSessionTTL)
	sessionID := uuid.New()
	var created repo.MusicPlaybackSession
	err = s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		var createErr error
		created, createErr = q.CreateMusicPlaybackSession(ctx, repo.CreateMusicPlaybackSessionParams{
			SessionID: sessionID, OwnerID: ownerID, SourceKind: source.Kind, SourceID: sourceID,
			SourceRevision: sourceRevision, ExpiresAt: dbtypes.NewTimestamp(expiresAt),
		})
		if createErr != nil {
			return createErr
		}
		for sequence, item := range items {
			if err := q.CreateMusicPlaybackEntry(ctx, repo.CreateMusicPlaybackEntryParams{
				EntryID: uuid.New().String(), SessionID: sessionID, Sequence: int64(sequence),
				TrackID: uuidNull(item.trackID), SourceEntryID: item.sourceEntryID, SavedTitle: item.savedTitle,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return MusicPlaybackSession{}, err
	}
	result := playbackSessionFromRow(created)
	result.TotalEntries = total
	return result, nil
}

func (s *musicService) ListPlaybackEntries(ctx context.Context, ownerID int32, sessionID uuid.UUID, limit, offset int) (MusicPlaybackPage, error) {
	limit, offset = clampMusicPage(limit, offset)
	session, err := s.readQueries.GetMusicPlaybackSession(ctx, repo.GetMusicPlaybackSessionParams{SessionID: sessionID, OwnerID: ownerID})
	if err != nil {
		return MusicPlaybackPage{}, err
	}
	if session.Status != "active" || !session.ExpiresAt.Valid || !session.ExpiresAt.Time.After(time.Now().UTC()) {
		return MusicPlaybackPage{}, ErrPlaybackExpired
	}
	rows, err := s.readQueries.ListMusicPlaybackEntries(ctx, repo.ListMusicPlaybackEntriesParams{
		SessionID: sessionID, OwnerID: ownerID, Limit: int64(limit), Offset: int64(offset),
	})
	if err != nil {
		return MusicPlaybackPage{}, err
	}
	total, err := s.readQueries.CountMusicPlaybackEntries(ctx, repo.CountMusicPlaybackEntriesParams{SessionID: sessionID, OwnerID: ownerID})
	if err != nil {
		return MusicPlaybackPage{}, err
	}
	items := make([]MusicPlaybackEntry, 0, len(rows))
	for _, row := range rows {
		items = append(items, playbackEntryFromRow(row))
	}
	return MusicPlaybackPage{Items: items, Total: total, Offset: offset, Limit: limit}, nil
}

func (s *musicService) ExpirePlaybackSession(ctx context.Context, ownerID int32, sessionID uuid.UUID) error {
	if _, err := s.readQueries.GetMusicPlaybackSession(ctx, repo.GetMusicPlaybackSessionParams{SessionID: sessionID, OwnerID: ownerID}); err != nil {
		return err
	}
	return s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		return q.ExpireMusicPlaybackSession(ctx, repo.ExpireMusicPlaybackSessionParams{SessionID: sessionID, OwnerID: ownerID})
	})
}

func (s *musicService) BackfillAll(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 500
	}
	if limit > 2000 {
		limit = 2000
	}
	owners, err := s.readQueries.ListMusicOwners(ctx)
	if err != nil {
		return err
	}
	for _, ownerID := range owners {
		owner := ownerID
		assets, listErr := s.readQueries.ListUncatalogedAudioAssets(ctx, repo.ListUncatalogedAudioAssetsParams{OwnerID: &owner, Limit: int64(limit)})
		if listErr != nil {
			return listErr
		}
		for _, asset := range assets {
			asset := asset
			if err := s.write(ctx, func(tx *sql.Tx, q *repo.Queries) error {
				return SyncMusicTrackFromAssetTx(ctx, tx, q, asset.AssetID, asset.SpecificMetadata)
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// Explicit artwork wins; otherwise use an available embedded cover from this
// owner's live album tracks. Reading a cover never mutates album metadata.
func (s *musicService) resolveAlbumCover(ctx context.Context, ownerID int32, album *MusicAlbum) error {
	if album.CoverAssetID != nil {
		return nil
	}
	id, err := s.readQueries.GetMusicAlbumAutomaticCover(ctx, repo.GetMusicAlbumAutomaticCoverParams{AlbumID: uuidNull(&album.AlbumID), OwnerID: ownerID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	album.CoverAssetID = &id
	return nil
}

func (s *musicService) GetLyrics(ctx context.Context, ownerID int32, trackID uuid.UUID) (MusicLyrics, error) {
	row, err := s.readQueries.GetMusicTrackLyrics(ctx, repo.GetMusicTrackLyricsParams{TrackID: trackID, OwnerID: ownerID})
	if err != nil {
		return MusicLyrics{}, err
	}
	return MusicLyrics{Content: row.Content, Revision: row.Revision}, nil
}

func (s *musicService) UpdateLyrics(ctx context.Context, ownerID int32, trackID uuid.UUID, content string, revision int64) (MusicLyrics, error) {
	if len(content) > 65536 || revision < 0 {
		return MusicLyrics{}, ErrMusicInvalid
	}
	err := s.write(ctx, func(_ *sql.Tx, q *repo.Queries) error {
		current, err := q.GetMusicTrackLyrics(ctx, repo.GetMusicTrackLyricsParams{TrackID: trackID, OwnerID: ownerID})
		if err != nil {
			return err
		}
		if current.Revision != revision {
			return ErrMusicConflict
		}
		return q.UpsertMusicTrackLyrics(ctx, repo.UpsertMusicTrackLyricsParams{TrackID: trackID.String(), Content: content})
	})
	if err != nil {
		return MusicLyrics{}, err
	}
	return s.GetLyrics(ctx, ownerID, trackID)
}
