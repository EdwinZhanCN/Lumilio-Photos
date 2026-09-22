package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MusicHandler exposes the owner-scoped music projection and its durable
// playlist/playback operations. It never accepts an owner id from the URL.
type MusicHandler struct {
	musicService service.MusicService
}

func NewMusicHandler(musicService service.MusicService) *MusicHandler {
	return &MusicHandler{musicService: musicService}
}

func writeMusicError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrMusicInvalid):
		api.WriteProblem(c, api.BadRequest(err))
	case errors.Is(err, service.ErrMusicConflict):
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
	case errors.Is(err, service.ErrPlaybackExpired):
		api.WriteProblem(c, api.StatusProblem(http.StatusGone, err))
	case errors.Is(err, service.ErrPlaybackTooLarge):
		api.WriteProblem(c, api.StatusProblem(http.StatusRequestEntityTooLarge, err))
	case errors.Is(err, service.ErrMusicUnavailable):
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
	case errors.Is(err, service.ErrMusicNotFound), errors.Is(err, sql.ErrNoRows):
		api.WriteProblem(c, api.NotFound(err))
	default:
		api.WriteProblem(c, api.Internal(err))
	}
}

func musicOwner(c *gin.Context) (int32, bool) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return 0, false
	}
	return int32(user.UserID), true
}

func parseMusicUUID(c *gin.Context, raw, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || id == uuid.Nil {
		if err == nil {
			err = errors.New("invalid " + name)
		}
		api.WriteProblem(c, api.BadRequest(err))
		return uuid.Nil, false
	}
	return id, true
}

func parseOptionalMusicUUID(c *gin.Context, raw *string, name string) (*uuid.UUID, bool) {
	if raw == nil {
		return nil, true
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		id := uuid.Nil
		return &id, true
	}
	id, ok := parseMusicUUID(c, trimmed, name)
	if !ok {
		return nil, false
	}
	return &id, true
}

func musicPagination(c *gin.Context) (int, int, bool) {
	limit, offset := parseListPagination(c, 50, 200)
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			api.WriteProblem(c, api.BadRequest(errors.New("limit must be between 1 and 200")))
			return 0, 0, false
		}
		limit = value
	}
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			api.WriteProblem(c, api.BadRequest(errors.New("offset must be non-negative")))
			return 0, 0, false
		}
		offset = value
	}
	return limit, offset, true
}

func toMusicCreditDTO(credit service.MusicCredit) dto.MusicCreditDTO {
	return dto.MusicCreditDTO{ArtistID: credit.ArtistID.String(), DisplayName: credit.DisplayName, Position: credit.Position, Role: credit.Role}
}

func toMusicTrackDTO(track service.MusicTrack) dto.MusicTrackDTO {
	result := dto.MusicTrackDTO{
		TrackID: track.TrackID.String(), OwnerID: track.OwnerID, Designation: track.Designation,
		Title: track.Title, AlbumTitle: track.AlbumTitle, ArtistName: track.ArtistName,
		AlbumArtistName: track.AlbumArtistName, Genre: track.Genre, ReleaseDate: track.ReleaseDate,
		ReleasePrecision: track.ReleasePrecision, Edition: track.Edition, ReleaseIdentifier: track.ReleaseIdentifier,
		DiscNumber: track.DiscNumber, DiscTotal: track.DiscTotal, TrackNumber: track.TrackNumber,
		TrackTotal: track.TrackTotal, Compilation: track.Compilation, ExtractedSourceRevision: track.ExtractedSourceRevision,
		Revision: track.Revision, OriginalFilename: track.OriginalFilename, MimeType: track.MimeType,
		Duration: track.Duration, TakenAt: track.TakenAt, IsDeleted: track.IsDeleted, Liked: track.Liked, Rating: track.Rating,
	}
	if track.AlbumID != nil {
		value := track.AlbumID.String()
		result.AlbumID = &value
	}
	if len(track.Artists) > 0 {
		result.Artists = make([]dto.MusicCreditDTO, 0, len(track.Artists))
		for _, credit := range track.Artists {
			result.Artists = append(result.Artists, toMusicCreditDTO(credit))
		}
	}
	if len(track.Overrides) > 0 {
		result.Overrides = make([]dto.MusicOverrideDTO, 0, len(track.Overrides))
		for _, override := range track.Overrides {
			result.Overrides = append(result.Overrides, dto.MusicOverrideDTO{Field: override.Field, Value: override.Value, Present: override.Present})
		}
	}
	return result
}

func toMusicTrackPageDTO(page service.MusicTrackPage) dto.MusicTrackPageDTO {
	result := dto.MusicTrackPageDTO{Total: page.Total, Offset: page.Offset, Limit: page.Limit, Items: make([]dto.MusicTrackDTO, 0, len(page.Items))}
	for _, track := range page.Items {
		result.Items = append(result.Items, toMusicTrackDTO(track))
	}
	return result
}

func toMusicAlbumDTO(album service.MusicAlbum) dto.MusicAlbumDTO {
	result := dto.MusicAlbumDTO{
		AlbumID: album.AlbumID.String(), OwnerID: album.OwnerID, Title: album.Title,
		ReleaseDate: album.ReleaseDate, ReleasePrecision: album.ReleasePrecision, Edition: album.Edition,
		Favorite: album.Favorite, ReleaseIdentifier: album.ReleaseIdentifier, Revision: album.Revision, TrackCount: album.TrackCount,
	}
	if album.CoverAssetID != nil {
		value := album.CoverAssetID.String()
		result.CoverAssetID = &value
	}
	for _, credit := range album.Artists {
		result.Artists = append(result.Artists, toMusicCreditDTO(credit))
	}
	for _, track := range album.Tracks {
		result.Tracks = append(result.Tracks, toMusicTrackDTO(track))
	}
	return result
}

func toMusicAlbumPageDTO(page service.MusicAlbumPage) dto.MusicAlbumPageDTO {
	result := dto.MusicAlbumPageDTO{Total: page.Total, Offset: page.Offset, Limit: page.Limit, Items: make([]dto.MusicAlbumDTO, 0, len(page.Items))}
	for _, album := range page.Items {
		result.Items = append(result.Items, toMusicAlbumDTO(album))
	}
	return result
}

func toMusicArtistDTO(artist service.MusicArtist) dto.MusicArtistDTO {
	return dto.MusicArtistDTO{Favorite: artist.Favorite, ArtistID: artist.ArtistID.String(), OwnerID: artist.OwnerID, DisplayName: artist.DisplayName, Revision: artist.Revision, TrackCount: artist.TrackCount, AlbumCount: artist.AlbumCount}
}

func toMusicArtistPageDTO(page service.MusicArtistPage) dto.MusicArtistPageDTO {
	result := dto.MusicArtistPageDTO{Total: page.Total, Offset: page.Offset, Limit: page.Limit, Items: make([]dto.MusicArtistDTO, 0, len(page.Items))}
	for _, artist := range page.Items {
		result.Items = append(result.Items, toMusicArtistDTO(artist))
	}
	return result
}

func toMusicPlaylistDTO(playlist service.MusicPlaylist) dto.MusicPlaylistDTO {
	return dto.MusicPlaylistDTO{PlaylistID: playlist.PlaylistID.String(), OwnerID: playlist.OwnerID, Title: playlist.Title, Description: playlist.Description, Revision: playlist.Revision, EntryCount: playlist.EntryCount, CoverAssetID: playlist.CoverAssetID}
}

func toMusicPlaylistPageDTO(page service.MusicPlaylistPage) dto.MusicPlaylistPageDTO {
	result := dto.MusicPlaylistPageDTO{Total: page.Total, Offset: page.Offset, Limit: page.Limit, Items: make([]dto.MusicPlaylistDTO, 0, len(page.Items))}
	for _, playlist := range page.Items {
		result.Items = append(result.Items, toMusicPlaylistDTO(playlist))
	}
	return result
}

func toMusicPlaylistEntryDTO(entry service.MusicPlaylistEntry) dto.MusicPlaylistEntryDTO {
	result := dto.MusicPlaylistEntryDTO{EntryID: entry.EntryID, PlaylistID: entry.PlaylistID.String(), SavedTitle: entry.SavedTitle, Position: entry.Position, IdempotencyKey: entry.IdempotencyKey, Available: entry.Available}
	if entry.TrackID != nil {
		value := entry.TrackID.String()
		result.TrackID = &value
	}
	if entry.Track != nil {
		track := toMusicTrackDTO(*entry.Track)
		result.Track = &track
	}
	return result
}

func toMusicPlaybackSessionDTO(session service.MusicPlaybackSession) dto.MusicPlaybackSessionDTO {
	return dto.MusicPlaybackSessionDTO{SessionID: session.SessionID.String(), OwnerID: session.OwnerID, SourceKind: session.SourceKind, SourceID: session.SourceID, SourceRevision: session.SourceRevision, Status: session.Status, ExpiresAt: session.ExpiresAt, TotalEntries: session.TotalEntries}
}

func toMusicPlaybackPageDTO(page service.MusicPlaybackPage) dto.MusicPlaybackPageDTO {
	result := dto.MusicPlaybackPageDTO{Total: page.Total, Offset: page.Offset, Limit: page.Limit, Items: make([]dto.MusicPlaybackEntryDTO, 0, len(page.Items))}
	for _, entry := range page.Items {
		item := dto.MusicPlaybackEntryDTO{EntryID: entry.EntryID, Sequence: entry.Sequence, SavedTitle: entry.SavedTitle, Available: entry.Available, TrackTitle: entry.TrackTitle, TrackArtist: entry.TrackArtist, TrackAlbum: entry.TrackAlbum, MimeType: entry.MimeType, Duration: entry.Duration, SourceEntryID: entry.SourceEntryID}
		if entry.TrackID != nil {
			value := entry.TrackID.String()
			item.TrackID = &value
		}
		result.Items = append(result.Items, item)
	}
	return result
}

// ListTracks lists the current user's first-class audio tracks.
// @Summary List music tracks
// @Tags music
// @Produce json
// @Param query query string false "Search title, artist, album, or filename"
// @Param sort query string false "Sort by title, artist, album, or track"
// @Param liked_only query bool false "Only liked tracks"
// @Param artist_id query string false "Filter by artist identity"
// @Param limit query int false "Maximum number of results"
// @Param offset query int false "Number of results to skip"
// @Success 200 {object} dto.MusicTrackPageDTO
// @Failure 401 {object} api.ProblemResponse
// @Router /api/v1/music/tracks [get]
// @Security BearerAuth
func (h *MusicHandler) ListTracks(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	limit, offset, ok := musicPagination(c)
	if !ok {
		return
	}
	likedOnly := false
	if raw := strings.TrimSpace(c.Query("liked_only")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		likedOnly = parsed
	}
	page, err := h.musicService.ListTracks(c.Request.Context(), ownerID, c.Query("query"), c.Query("sort"), c.Query("artist_id"), likedOnly, limit, offset)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicTrackPageDTO(page))
}

// GetTrack returns one owner-scoped music track.
// @Summary Get music track
// @Tags music
// @Produce json
// @Param id path string true "Track UUID"
// @Success 200 {object} dto.MusicTrackDTO
// @Failure 404 {object} api.ProblemResponse
// @Router /api/v1/music/tracks/{id} [get]
// @Security BearerAuth
func (h *MusicHandler) GetTrack(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	trackID, ok := parseMusicUUID(c, c.Param("id"), "track id")
	if !ok {
		return
	}
	track, err := h.musicService.GetTrack(c.Request.Context(), ownerID, trackID)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicTrackDTO(track))
}

// UpdateTrack applies field-level overrides to a music track.
// @Summary Update music track
// @Tags music
// @Accept json
// @Produce json
// @Param id path string true "Track UUID"
// @Param request body dto.MusicTrackPatchRequestDTO true "Track changes"
// @Success 200 {object} dto.MusicTrackDTO
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/music/tracks/{id} [patch]
// @Security BearerAuth
func (h *MusicHandler) UpdateTrack(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	trackID, ok := parseMusicUUID(c, c.Param("id"), "track id")
	if !ok {
		return
	}
	var request dto.MusicTrackPatchRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	albumID, ok := parseOptionalMusicUUID(c, request.AlbumID, "album id")
	if !ok {
		return
	}
	patch := service.MusicTrackPatch{
		Title: request.Title, AlbumTitle: request.AlbumTitle, ArtistName: request.ArtistName,
		AlbumArtistName: request.AlbumArtistName, Genre: request.Genre, ReleaseDate: request.ReleaseDate,
		ReleasePrecision: request.ReleasePrecision, Edition: request.Edition, DiscNumber: request.DiscNumber,
		DiscTotal: request.DiscTotal, TrackNumber: request.TrackNumber, TrackTotal: request.TrackTotal,
		Compilation: request.Compilation, Designation: request.Designation, AlbumID: albumID,
		Artists: request.ArtistNames, AlbumArtists: request.AlbumArtistNames,
	}
	if request.AlbumID == nil {
		patch.AlbumID = nil
	}
	track, err := h.musicService.UpdateTrack(c.Request.Context(), ownerID, trackID, patch, request.Revision)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicTrackDTO(track))
}

// ResetTrackOverrides restores all effective fields from the extracted source.
// @Summary Reset music track overrides
// @Tags music
// @Produce json
// @Param id path string true "Track UUID"
// @Success 200 {object} dto.MusicTrackDTO
// @Router /api/v1/music/tracks/{id}/reset-overrides [post]
// @Security BearerAuth
func (h *MusicHandler) ResetTrackOverrides(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	trackID, ok := parseMusicUUID(c, c.Param("id"), "track id")
	if !ok {
		return
	}
	track, err := h.musicService.ResetTrackOverrides(c.Request.Context(), ownerID, trackID)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicTrackDTO(track))
}

// SetTrackDesignation changes whether a track participates in music views.
// @Summary Set music track designation
// @Tags music
// @Accept json
// @Produce json
// @Param id path string true "Track UUID"
// @Param request body dto.MusicDesignationRequestDTO true "Designation change"
// @Success 200 {object} dto.MusicTrackDTO
// @Router /api/v1/music/tracks/{id}/designation [put]
// @Security BearerAuth
func (h *MusicHandler) SetTrackDesignation(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	trackID, ok := parseMusicUUID(c, c.Param("id"), "track id")
	if !ok {
		return
	}
	var request dto.MusicDesignationRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	track, err := h.musicService.SetTrackDesignation(c.Request.Context(), ownerID, trackID, request.Designation, request.Revision)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicTrackDTO(track))
}

// ListAlbums lists first-class music albums, independent of legacy asset albums.
// @Summary List music albums
// @Tags music
// @Produce json
// @Param query query string false "Search album title"
// @Param favorites_only query bool false "Only favorite albums"
// @Param limit query int false "Maximum number of results"
// @Param offset query int false "Number of results to skip"
// @Success 200 {object} dto.MusicAlbumPageDTO
// @Router /api/v1/music/albums [get]
// @Security BearerAuth
func (h *MusicHandler) ListAlbums(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	limit, offset, ok := musicPagination(c)
	if !ok {
		return
	}
	page, err := h.musicService.ListAlbums(c.Request.Context(), ownerID, c.Query("query"), c.Query("favorites_only") == "true", limit, offset)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicAlbumPageDTO(page))
}

// GetAlbum returns a first-class music album and its tracks.
// @Summary Get music album
// @Tags music
// @Produce json
// @Param id path string true "Album UUID"
// @Success 200 {object} dto.MusicAlbumDTO
// @Router /api/v1/music/albums/{id} [get]
// @Security BearerAuth
func (h *MusicHandler) GetAlbum(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	albumID, ok := parseMusicUUID(c, c.Param("id"), "album id")
	if !ok {
		return
	}
	album, err := h.musicService.GetAlbum(c.Request.Context(), ownerID, albumID)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicAlbumDTO(album))
}

// CreateAlbum creates a first-class music album.
// @Summary Create music album
// @Tags music
// @Accept json
// @Produce json
// @Param request body dto.MusicAlbumCreateRequestDTO true "Album data"
// @Success 200 {object} dto.MusicAlbumDTO
// @Router /api/v1/music/albums [post]
// @Security BearerAuth
func (h *MusicHandler) CreateAlbum(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	var request dto.MusicAlbumCreateRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	coverAssetID, ok := parseOptionalMusicUUID(c, request.CoverAssetID, "cover asset id")
	if !ok {
		return
	}
	album, err := h.musicService.CreateAlbum(c.Request.Context(), ownerID, request.Title, request.ReleaseDate, request.ReleasePrecision, request.Edition, coverAssetID, request.ArtistNames)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicAlbumDTO(album))
}

// UpdateAlbum updates a music album using optimistic concurrency.
// @Summary Update music album
// @Tags music
// @Accept json
// @Produce json
// @Param id path string true "Album UUID"
// @Param request body dto.MusicAlbumPatchRequestDTO true "Album changes"
// @Success 200 {object} dto.MusicAlbumDTO
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/music/albums/{id} [patch]
// @Security BearerAuth
func (h *MusicHandler) UpdateAlbum(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	albumID, ok := parseMusicUUID(c, c.Param("id"), "album id")
	if !ok {
		return
	}
	var request dto.MusicAlbumPatchRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	coverAssetID, ok := parseOptionalMusicUUID(c, request.CoverAssetID, "cover asset id")
	if !ok {
		return
	}
	patch := service.MusicAlbumPatch{Favorite: request.Favorite, Title: request.Title, ReleaseDate: request.ReleaseDate, ReleasePrecision: request.ReleasePrecision, Edition: request.Edition, ArtistNames: request.ArtistNames}
	if request.CoverAssetID != nil {
		patch.CoverAssetID = coverAssetID
	}
	album, err := h.musicService.UpdateAlbum(c.Request.Context(), ownerID, albumID, patch, request.Revision)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicAlbumDTO(album))
}

// AssignTrackAlbum assigns or clears a track's first-class music album.
// @Summary Assign music album to track
// @Tags music
// @Accept json
// @Produce json
// @Param id path string true "Track UUID"
// @Param request body dto.MusicAlbumAssignmentRequestDTO true "Album assignment"
// @Success 204
// @Router /api/v1/music/tracks/{id}/album [put]
// @Security BearerAuth
func (h *MusicHandler) AssignTrackAlbum(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	trackID, ok := parseMusicUUID(c, c.Param("id"), "track id")
	if !ok {
		return
	}
	var request dto.MusicAlbumAssignmentRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	albumID, ok := parseOptionalMusicUUID(c, request.AlbumID, "album id")
	if !ok {
		return
	}
	if request.AlbumID == nil {
		cleared := uuid.Nil
		albumID = &cleared
	}
	if err := h.musicService.AssignTrackAlbum(c.Request.Context(), ownerID, trackID, albumID, request.Revision); err != nil {
		writeMusicError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListArtists lists the owner-scoped music artist vocabulary.
// @Param favorites_only query bool false "Only favorite artists"
// @Summary List music artists
// @Tags music
// @Produce json
// @Param query query string false "Search artist name"
// @Param limit query int false "Maximum number of results"
// @Param offset query int false "Number of results to skip"
// @Success 200 {object} dto.MusicArtistPageDTO
// @Router /api/v1/music/artists [get]
// @Security BearerAuth
func (h *MusicHandler) ListArtists(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	limit, offset, ok := musicPagination(c)
	if !ok {
		return
	}
	page, err := h.musicService.ListArtists(c.Request.Context(), ownerID, c.Query("query"), c.Query("favorites_only") == "true", limit, offset)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicArtistPageDTO(page))
}

// GetArtist returns one owner-scoped music artist.
// @Summary Get music artist
// @Tags music
// @Produce json
// @Param id path string true "Artist UUID"
// @Success 200 {object} dto.MusicArtistDTO
// @Router /api/v1/music/artists/{id} [get]
// @Security BearerAuth
func (h *MusicHandler) GetArtist(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	artistID, ok := parseMusicUUID(c, c.Param("id"), "artist id")
	if !ok {
		return
	}
	artist, err := h.musicService.GetArtist(c.Request.Context(), ownerID, artistID)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicArtistDTO(artist))
}

// UpdateArtist renames an owner-scoped music artist.
// @Summary Update music artist
// @Tags music
// @Accept json
// @Produce json
// @Param id path string true "Artist UUID"
// @Param request body dto.MusicArtistPatchRequestDTO true "Artist changes"
// @Success 200 {object} dto.MusicArtistDTO
// @Router /api/v1/music/artists/{id} [patch]
// @Security BearerAuth
func (h *MusicHandler) UpdateArtist(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	artistID, ok := parseMusicUUID(c, c.Param("id"), "artist id")
	if !ok {
		return
	}
	var request dto.MusicArtistPatchRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	artist, err := h.musicService.UpdateArtist(c.Request.Context(), ownerID, artistID, service.MusicArtistPatch{Favorite: request.Favorite, DisplayName: request.DisplayName}, request.Revision)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicArtistDTO(artist))
}

// ListPlaylists lists the current user's durable music playlists.
// @Summary List music playlists
// @Tags music
// @Produce json
// @Param limit query int false "Maximum number of results"
// @Param offset query int false "Number of results to skip"
// @Success 200 {object} dto.MusicPlaylistPageDTO
// @Router /api/v1/music/playlists [get]
// @Security BearerAuth
func (h *MusicHandler) ListPlaylists(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	limit, offset, ok := musicPagination(c)
	if !ok {
		return
	}
	page, err := h.musicService.ListPlaylists(c.Request.Context(), ownerID, limit, offset)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicPlaylistPageDTO(page))
}

// CreatePlaylist creates a durable ordered playlist.
// @Summary Create music playlist
// @Tags music
// @Accept json
// @Produce json
// @Param request body dto.MusicPlaylistCreateRequestDTO true "Playlist data"
// @Success 200 {object} dto.MusicPlaylistDTO
// @Router /api/v1/music/playlists [post]
// @Security BearerAuth
func (h *MusicHandler) CreatePlaylist(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	var request dto.MusicPlaylistCreateRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	playlist, err := h.musicService.CreatePlaylist(c.Request.Context(), ownerID, request.Title, request.Description)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicPlaylistDTO(playlist))
}

// GetPlaylist returns a durable playlist summary.
// @Summary Get music playlist
// @Tags music
// @Produce json
// @Param id path string true "Playlist UUID"
// @Success 200 {object} dto.MusicPlaylistDTO
// @Router /api/v1/music/playlists/{id} [get]
// @Security BearerAuth
func (h *MusicHandler) GetPlaylist(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	playlistID, ok := parseMusicUUID(c, c.Param("id"), "playlist id")
	if !ok {
		return
	}
	playlist, err := h.musicService.GetPlaylist(c.Request.Context(), ownerID, playlistID)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicPlaylistDTO(playlist))
}

// UpdatePlaylist updates playlist metadata using optimistic concurrency.
// @Summary Update music playlist
// @Tags music
// @Accept json
// @Produce json
// @Param id path string true "Playlist UUID"
// @Param request body dto.MusicPlaylistPatchRequestDTO true "Playlist changes"
// @Success 200 {object} dto.MusicPlaylistDTO
// @Router /api/v1/music/playlists/{id} [patch]
// @Security BearerAuth
func (h *MusicHandler) UpdatePlaylist(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	playlistID, ok := parseMusicUUID(c, c.Param("id"), "playlist id")
	if !ok {
		return
	}
	var request dto.MusicPlaylistPatchRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	playlist, err := h.musicService.UpdatePlaylist(c.Request.Context(), ownerID, playlistID, request.Title, request.Description, request.Revision)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicPlaylistDTO(playlist))
}

// DeletePlaylist deletes a durable playlist owned by the current user.
// @Summary Delete music playlist
// @Tags music
// @Param id path string true "Playlist UUID"
// @Success 204
// @Router /api/v1/music/playlists/{id} [delete]
// @Security BearerAuth
func (h *MusicHandler) DeletePlaylist(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	playlistID, ok := parseMusicUUID(c, c.Param("id"), "playlist id")
	if !ok {
		return
	}
	if err := h.musicService.DeletePlaylist(c.Request.Context(), ownerID, playlistID); err != nil {
		writeMusicError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListPlaylistEntries lists ordered entries, including tombstones for missing tracks.
// @Summary List music playlist entries
// @Tags music
// @Produce json
// @Param id path string true "Playlist UUID"
// @Success 200 {object} dto.MusicPlaylistEntriesResponseDTO
// @Router /api/v1/music/playlists/{id}/entries [get]
// @Security BearerAuth
func (h *MusicHandler) ListPlaylistEntries(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	playlistID, ok := parseMusicUUID(c, c.Param("id"), "playlist id")
	if !ok {
		return
	}
	playlist, err := h.musicService.GetPlaylist(c.Request.Context(), ownerID, playlistID)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	entries, err := h.musicService.ListPlaylistEntries(c.Request.Context(), ownerID, playlistID)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	result := dto.MusicPlaylistEntriesResponseDTO{Revision: playlist.Revision, Items: make([]dto.MusicPlaylistEntryDTO, 0, len(entries))}
	for _, entry := range entries {
		result.Items = append(result.Items, toMusicPlaylistEntryDTO(entry))
	}
	api.JSONOK(c, result)
}

// AddPlaylistEntry appends an entry with an optional idempotency key.
// @Summary Add music playlist entry
// @Tags music
// @Accept json
// @Produce json
// @Param id path string true "Playlist UUID"
// @Param request body dto.MusicPlaylistEntryCreateRequestDTO true "Entry data"
// @Success 200 {object} dto.MusicPlaylistEntryDTO
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/music/playlists/{id}/entries [post]
// @Security BearerAuth
func (h *MusicHandler) AddPlaylistEntry(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	playlistID, ok := parseMusicUUID(c, c.Param("id"), "playlist id")
	if !ok {
		return
	}
	var request dto.MusicPlaylistEntryCreateRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	trackID, ok := parseMusicUUID(c, request.TrackID, "track id")
	if !ok {
		return
	}
	entry, err := h.musicService.AddPlaylistEntry(c.Request.Context(), ownerID, playlistID, trackID, musicStringValue(request.IdempotencyKey), request.SavedTitle, request.Revision)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicPlaylistEntryDTO(entry))
}

func musicStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// RemovePlaylistEntry removes and compacts a playlist.
// @Summary Remove music playlist entry
// @Tags music
// @Param id path string true "Playlist UUID"
// @Param entryId path string true "Entry UUID"
// @Param revision query int false "Expected playlist revision"
// @Success 204
// @Router /api/v1/music/playlists/{id}/entries/{entryId} [delete]
// @Security BearerAuth
func (h *MusicHandler) RemovePlaylistEntry(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	playlistID, ok := parseMusicUUID(c, c.Param("id"), "playlist id")
	if !ok {
		return
	}
	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil && c.Query("revision") != "" {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if err := h.musicService.RemovePlaylistEntry(c.Request.Context(), ownerID, playlistID, c.Param("entryId"), revision); err != nil {
		writeMusicError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ReorderPlaylist replaces the complete ordered entry set with a CAS revision.
// @Summary Reorder music playlist
// @Tags music
// @Accept json
// @Param id path string true "Playlist UUID"
// @Param request body dto.MusicPlaylistReorderRequestDTO true "Entry positions"
// @Success 204
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/music/playlists/{id}/entries/reorder [put]
// @Security BearerAuth
func (h *MusicHandler) ReorderPlaylist(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	playlistID, ok := parseMusicUUID(c, c.Param("id"), "playlist id")
	if !ok {
		return
	}
	var request dto.MusicPlaylistReorderRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	positions := make([]service.MusicEntryPosition, 0, len(request.Entries))
	for _, item := range request.Entries {
		positions = append(positions, service.MusicEntryPosition{EntryID: item.EntryID, Position: item.Position})
	}
	if err := h.musicService.ReorderPlaylist(c.Request.Context(), ownerID, playlistID, positions, request.Revision); err != nil {
		writeMusicError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CreatePlaybackSession snapshots a bounded source for stable queue paging.
// @Summary Create music playback session
// @Tags music
// @Accept json
// @Produce json
// @Param request body dto.MusicPlaybackSourceRequestDTO true "Playback source"
// @Success 200 {object} dto.MusicPlaybackSessionDTO
// @Failure 413 {object} api.ProblemResponse
// @Router /api/v1/music/playback-sessions [post]
// @Security BearerAuth
func (h *MusicHandler) CreatePlaybackSession(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	var request dto.MusicPlaybackSourceRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	session, err := h.musicService.CreatePlaybackSession(c.Request.Context(), ownerID, service.PlaybackSource{Sort: request.Sort, ArtistID: request.ArtistID, Kind: request.Kind, ID: request.ID, Query: request.Query, LikedOnly: request.LikedOnly})
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicPlaybackSessionDTO(session))
}

// ListPlaybackEntries returns one page from a non-expired playback snapshot.
// @Summary List playback entries
// @Tags music
// @Produce json
// @Param id path string true "Session UUID"
// @Param limit query int false "Maximum number of results"
// @Param offset query int false "Number of results to skip"
// @Success 200 {object} dto.MusicPlaybackPageDTO
// @Failure 410 {object} api.ProblemResponse
// @Router /api/v1/music/playback-sessions/{id}/entries [get]
// @Security BearerAuth
func (h *MusicHandler) ListPlaybackEntries(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	sessionID, ok := parseMusicUUID(c, c.Param("id"), "session id")
	if !ok {
		return
	}
	limit, offset, ok := musicPagination(c)
	if !ok {
		return
	}
	page, err := h.musicService.ListPlaybackEntries(c.Request.Context(), ownerID, sessionID, limit, offset)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, toMusicPlaybackPageDTO(page))
}

// ExpirePlaybackSession explicitly expires a playback snapshot.
// @Summary Expire playback session
// @Tags music
// @Param id path string true "Session UUID"
// @Success 204
// @Router /api/v1/music/playback-sessions/{id} [delete]
// @Security BearerAuth
func (h *MusicHandler) ExpirePlaybackSession(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	sessionID, ok := parseMusicUUID(c, c.Param("id"), "session id")
	if !ok {
		return
	}
	if err := h.musicService.ExpirePlaybackSession(c.Request.Context(), ownerID, sessionID); err != nil {
		writeMusicError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetLyrics reads locally stored lyrics.
// @Summary Get local track lyrics
// @Tags music
// @Produce json
// @Param id path string true "Track UUID"
// @Success 200 {object} dto.MusicLyricsDTO
// @Router /api/v1/music/tracks/{id}/lyrics [get]
// @Security BearerAuth
func (h *MusicHandler) GetLyrics(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	id, ok := parseMusicUUID(c, c.Param("id"), "track id")
	if !ok {
		return
	}
	lyrics, err := h.musicService.GetLyrics(c.Request.Context(), ownerID, id)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, dto.MusicLyricsDTO{Content: lyrics.Content, Revision: lyrics.Revision})
}

// UpdateLyrics stores user-provided local text without modifying original audio.
// @Summary Save local track lyrics
// @Tags music
// @Accept json
// @Produce json
// @Param id path string true "Track UUID"
// @Param request body dto.MusicLyricsDTO true "Local lyrics and expected revision"
// @Success 200 {object} dto.MusicLyricsDTO
// @Router /api/v1/music/tracks/{id}/lyrics [put]
// @Security BearerAuth
func (h *MusicHandler) UpdateLyrics(c *gin.Context) {
	ownerID, ok := musicOwner(c)
	if !ok {
		return
	}
	id, ok := parseMusicUUID(c, c.Param("id"), "track id")
	if !ok {
		return
	}
	var request dto.MusicLyricsDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	lyrics, err := h.musicService.UpdateLyrics(c.Request.Context(), ownerID, id, request.Content, request.Revision)
	if err != nil {
		writeMusicError(c, err)
		return
	}
	api.JSONOK(c, dto.MusicLyricsDTO{Content: lyrics.Content, Revision: lyrics.Revision})
}
