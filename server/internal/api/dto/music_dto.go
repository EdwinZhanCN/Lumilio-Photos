package dto

import "time"

// MusicCreditDTO is an ordered artist credit attached to a track or album.
type MusicCreditDTO struct {
	ArtistID    string `json:"artist_id" format:"uuid"`
	DisplayName string `json:"display_name"`
	Position    int64  `json:"position"`
	Role        string `json:"role"`
}

type MusicOverrideDTO struct {
	Field   string  `json:"field"`
	Value   *string `json:"value,omitempty"`
	Present bool    `json:"present"`
}

type MusicTrackDTO struct {
	Rating                  *int64             `json:"rating,omitempty"`
	TrackID                 string             `json:"track_id" format:"uuid"`
	OwnerID                 int32              `json:"owner_id"`
	Designation             string             `json:"designation" enums:"music,other"`
	AlbumID                 *string            `json:"album_id,omitempty" format:"uuid"`
	Title                   string             `json:"title"`
	AlbumTitle              string             `json:"album_title"`
	ArtistName              string             `json:"artist_name"`
	AlbumArtistName         string             `json:"album_artist_name"`
	Genre                   string             `json:"genre"`
	ReleaseDate             *string            `json:"release_date,omitempty"`
	ReleasePrecision        string             `json:"release_precision" enums:"unknown,year,month,day"`
	Edition                 string             `json:"edition"`
	ReleaseIdentifier       string             `json:"release_identifier"`
	DiscNumber              *int64             `json:"disc_number,omitempty"`
	DiscTotal               *int64             `json:"disc_total,omitempty"`
	TrackNumber             *int64             `json:"track_number,omitempty"`
	TrackTotal              *int64             `json:"track_total,omitempty"`
	Compilation             bool               `json:"compilation"`
	ExtractedSourceRevision int64              `json:"extracted_source_revision"`
	Revision                int64              `json:"revision"`
	OriginalFilename        string             `json:"original_filename"`
	MimeType                string             `json:"mime_type"`
	Duration                *float64           `json:"duration,omitempty"`
	TakenAt                 *time.Time         `json:"taken_at,omitempty"`
	IsDeleted               bool               `json:"is_deleted"`
	Liked                   bool               `json:"liked"`
	Artists                 []MusicCreditDTO   `json:"artists,omitempty"`
	Overrides               []MusicOverrideDTO `json:"overrides,omitempty"`
}

type MusicTrackPageDTO struct {
	Items  []MusicTrackDTO `json:"items"`
	Total  int64           `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
}

type MusicTrackPatchRequestDTO struct {
	Title            *string   `json:"title,omitempty"`
	AlbumTitle       *string   `json:"album_title,omitempty"`
	ArtistName       *string   `json:"artist_name,omitempty"`
	AlbumArtistName  *string   `json:"album_artist_name,omitempty"`
	Genre            *string   `json:"genre,omitempty"`
	ReleaseDate      *string   `json:"release_date,omitempty"`
	ReleasePrecision *string   `json:"release_precision,omitempty"`
	Edition          *string   `json:"edition,omitempty"`
	DiscNumber       *int64    `json:"disc_number,omitempty"`
	DiscTotal        *int64    `json:"disc_total,omitempty"`
	TrackNumber      *int64    `json:"track_number,omitempty"`
	TrackTotal       *int64    `json:"track_total,omitempty"`
	Compilation      *bool     `json:"compilation,omitempty"`
	Designation      *string   `json:"designation,omitempty"`
	AlbumID          *string   `json:"album_id,omitempty" format:"uuid"`
	ArtistNames      *[]string `json:"artist_names,omitempty"`
	AlbumArtistNames *[]string `json:"album_artist_names,omitempty"`
	Revision         int64     `json:"revision"`
}

type MusicDesignationRequestDTO struct {
	Designation string `json:"designation" enums:"music,other" binding:"required,oneof=music other"`
	Revision    int64  `json:"revision"`
}

type MusicAlbumDTO struct {
	Favorite          bool             `json:"favorite"`
	AlbumID           string           `json:"album_id" format:"uuid"`
	OwnerID           int32            `json:"owner_id"`
	Title             string           `json:"title"`
	ReleaseDate       *string          `json:"release_date,omitempty"`
	ReleasePrecision  string           `json:"release_precision" enums:"unknown,year,month,day"`
	Edition           string           `json:"edition"`
	ReleaseIdentifier string           `json:"release_identifier"`
	CoverAssetID      *string          `json:"cover_asset_id,omitempty" format:"uuid"`
	Revision          int64            `json:"revision"`
	TrackCount        int64            `json:"track_count"`
	Artists           []MusicCreditDTO `json:"artists,omitempty"`
	Tracks            []MusicTrackDTO  `json:"tracks,omitempty"`
}

type MusicAlbumPageDTO struct {
	Items  []MusicAlbumDTO `json:"items"`
	Total  int64           `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
}

type MusicAlbumCreateRequestDTO struct {
	Title            string   `json:"title" binding:"required"`
	ReleaseDate      *string  `json:"release_date,omitempty"`
	ReleasePrecision string   `json:"release_precision,omitempty"`
	Edition          string   `json:"edition,omitempty"`
	CoverAssetID     *string  `json:"cover_asset_id,omitempty" format:"uuid"`
	ArtistNames      []string `json:"artist_names,omitempty"`
}

type MusicAlbumPatchRequestDTO struct {
	Favorite         *bool     `json:"favorite,omitempty"`
	Title            *string   `json:"title,omitempty"`
	ReleaseDate      *string   `json:"release_date,omitempty"`
	ReleasePrecision *string   `json:"release_precision,omitempty"`
	Edition          *string   `json:"edition,omitempty"`
	CoverAssetID     *string   `json:"cover_asset_id,omitempty" format:"uuid"`
	ArtistNames      *[]string `json:"artist_names,omitempty"`
	Revision         int64     `json:"revision"`
}

type MusicAlbumAssignmentRequestDTO struct {
	AlbumID  *string `json:"album_id,omitempty" format:"uuid"`
	Revision int64   `json:"revision"`
}

type MusicArtistDTO struct {
	Favorite    bool   `json:"favorite"`
	ArtistID    string `json:"artist_id" format:"uuid"`
	OwnerID     int32  `json:"owner_id"`
	DisplayName string `json:"display_name"`
	Revision    int64  `json:"revision"`
	TrackCount  int64  `json:"track_count"`
	AlbumCount  int64  `json:"album_count"`
}

type MusicArtistPageDTO struct {
	Items  []MusicArtistDTO `json:"items"`
	Total  int64            `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
}

type MusicArtistPatchRequestDTO struct {
	Favorite    *bool   `json:"favorite,omitempty"`
	DisplayName *string `json:"display_name,omitempty" binding:"omitempty,min=1"`
	Revision    int64   `json:"revision"`
}

type MusicPlaylistDTO struct {
	CoverAssetID string `json:"cover_asset_id,omitempty"`
	PlaylistID   string `json:"playlist_id" format:"uuid"`
	OwnerID      int32  `json:"owner_id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Revision     int64  `json:"revision"`
	EntryCount   int64  `json:"entry_count"`
}

type MusicPlaylistPageDTO struct {
	Items  []MusicPlaylistDTO `json:"items"`
	Total  int64              `json:"total"`
	Offset int                `json:"offset"`
	Limit  int                `json:"limit"`
}

type MusicPlaylistCreateRequestDTO struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description,omitempty"`
}

type MusicPlaylistPatchRequestDTO struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description,omitempty"`
	Revision    int64  `json:"revision"`
}

type MusicPlaylistEntryDTO struct {
	EntryID        string         `json:"entry_id" format:"uuid"`
	PlaylistID     string         `json:"playlist_id" format:"uuid"`
	TrackID        *string        `json:"track_id,omitempty" format:"uuid"`
	SavedTitle     string         `json:"saved_title"`
	Position       int64          `json:"position"`
	IdempotencyKey *string        `json:"idempotency_key,omitempty"`
	Available      bool           `json:"available"`
	Track          *MusicTrackDTO `json:"track,omitempty"`
}

type MusicPlaylistEntriesResponseDTO struct {
	Items    []MusicPlaylistEntryDTO `json:"items"`
	Revision int64                   `json:"revision"`
}

type MusicPlaylistEntryCreateRequestDTO struct {
	TrackID        string  `json:"track_id" binding:"required" format:"uuid"`
	SavedTitle     *string `json:"saved_title,omitempty"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
	Revision       int64   `json:"revision"`
}

type MusicPlaylistEntryPositionDTO struct {
	EntryID  string `json:"entry_id" binding:"required" format:"uuid"`
	Position int64  `json:"position"`
}

type MusicPlaylistReorderRequestDTO struct {
	Revision int64                           `json:"revision"`
	Entries  []MusicPlaylistEntryPositionDTO `json:"entries" binding:"required"`
}

type MusicPlaybackSourceRequestDTO struct {
	Sort      string `json:"sort,omitempty" binding:"omitempty,oneof=title artist album track"`
	ArtistID  string `json:"artist_id,omitempty" binding:"omitempty,uuid"`
	Kind      string `json:"kind" binding:"required,oneof=query album playlist liked"`
	ID        string `json:"id,omitempty" format:"uuid"`
	Query     string `json:"query,omitempty"`
	LikedOnly bool   `json:"liked_only,omitempty"`
}

type MusicPlaybackSessionDTO struct {
	SessionID      string    `json:"session_id" format:"uuid"`
	OwnerID        int32     `json:"owner_id"`
	SourceKind     string    `json:"source_kind"`
	SourceID       string    `json:"source_id,omitempty"`
	SourceRevision int64     `json:"source_revision"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
	TotalEntries   int64     `json:"total_entries"`
}

type MusicPlaybackEntryDTO struct {
	EntryID       string   `json:"entry_id" format:"uuid"`
	Sequence      int64    `json:"sequence"`
	TrackID       *string  `json:"track_id,omitempty" format:"uuid"`
	SourceEntryID *string  `json:"source_entry_id,omitempty" format:"uuid"`
	SavedTitle    string   `json:"saved_title"`
	Available     bool     `json:"available"`
	TrackTitle    string   `json:"track_title,omitempty"`
	TrackArtist   string   `json:"track_artist,omitempty"`
	TrackAlbum    string   `json:"track_album,omitempty"`
	MimeType      string   `json:"mime_type,omitempty"`
	Duration      *float64 `json:"duration,omitempty"`
}

type MusicPlaybackPageDTO struct {
	Items  []MusicPlaybackEntryDTO `json:"items"`
	Total  int64                   `json:"total"`
	Offset int                     `json:"offset"`
	Limit  int                     `json:"limit"`
}

type MusicLyricsDTO struct {
	Content  string `json:"content" binding:"max=65536"`
	Revision int64  `json:"revision" binding:"min=0"`
}

// AgentMusicRefDTO hydrates a bounded selection in its immutable ref order.
type AgentMusicRefDTO struct {
	Tracks    []MusicTrackDTO `json:"tracks"`
	Total     int             `json:"total"`
	Truncated bool            `json:"truncated"`
}
