package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"server/internal/agent/pins"
	"server/internal/db/repo"
	"server/internal/secretbox"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	shareLinkTokenHashScope    = "share.token.hash.v1"
	shareLinkTokenBytes        = 32
	shareLinkDefaultExpiryDays = 30
	shareLinkMaxExpiryDays     = 365
	// ShareLinkMaxAssets bounds how many assets a single share snapshot may
	// resolve to, so zip downloads and public browse pages stay bounded.
	ShareLinkMaxAssets = 5000
)

// Errors returned by ShareLinkService. Handlers map these to HTTP responses;
// ResolvePublic intentionally collapses "expired"/"revoked"/"unknown token"
// into the same ErrShareLinkNotFound so public probing can't distinguish them.
var (
	ErrShareLinkNotFound     = errors.New("share link not found")
	ErrShareLinkTooLarge     = errors.New("share source resolves to too many assets")
	ErrShareLinkSourceEmpty  = errors.New("share source resolves to no assets")
	ErrShareLinkNotDeletable = errors.New("share link must be expired or revoked before it can be deleted")
	// ErrShareLinkInvalidSource wraps source_kind/source_ref validation
	// failures so handlers can map them to 400 instead of 500.
	ErrShareLinkInvalidSource = errors.New("invalid share source")
)

// ShareLinkCreateParams collects the inputs needed to resolve a source and
// create a share link. OwnerID is the concrete creating user (stored as
// share_links.owner_id); OwnerScope mirrors the ownerScopeID(c) convention
// used elsewhere (nil for admins = no ownership restriction on included
// assets, else must equal OwnerID) and is applied when resolving
// album/person/utility_query/asset_snapshot sources.
type ShareLinkCreateParams struct {
	OwnerID          int32
	OwnerScope       *int32
	Title            string
	Description      *string
	SourceKind       string
	SourceRef        *string
	ExplicitAssetIDs []uuid.UUID
	ExpiresInDays    int
	AllowDownload    bool
	IncludeOriginals bool
}

// ShareLinkUpdateParams is a partial patch to a share link's settings.
// ExtendDays, when set, moves expires_at to max(now, expires_at) + N days.
type ShareLinkUpdateParams struct {
	Title            *string
	Description      *string
	AllowDownload    *bool
	IncludeOriginals *bool
	ExtendDays       *int
}

// ShareLinkService resolves share sources into asset snapshots, issues and
// validates share tokens, and serves both owner-facing management operations
// and public (token-authorized) share operations.
type ShareLinkService interface {
	Create(ctx context.Context, params ShareLinkCreateParams) (repo.ShareLink, string, error)
	List(ctx context.Context, ownerID int32) ([]repo.ShareLink, error)
	Get(ctx context.Context, ownerID int32, shareID uuid.UUID) (repo.ShareLink, error)
	UpdateSettings(ctx context.Context, ownerID int32, shareID uuid.UUID, params ShareLinkUpdateParams) (repo.ShareLink, error)
	Revoke(ctx context.Context, ownerID int32, shareID uuid.UUID) (repo.ShareLink, error)
	Delete(ctx context.Context, ownerID int32, shareID uuid.UUID) error

	// ResolvePublic authorizes a raw share token: active status and
	// non-expired only. Every public handler must call this first.
	ResolvePublic(ctx context.Context, rawToken string) (repo.ShareLink, error)
	RecordView(ctx context.Context, shareID uuid.UUID) error
	// PublicAssetSource wraps a resolved share's asset snapshot for reuse with
	// AssetService.QueryBrowseItems, the same source-scoping mechanism pins use.
	PublicAssetSource(link repo.ShareLink) *AssetSetSource
	// AssetInShare is the membership check every public media handler must
	// pass before serving a specific asset's thumbnail/video/audio/original.
	AssetInShare(link repo.ShareLink, assetID uuid.UUID) bool
}

type shareLinkService struct {
	queries      *repo.Queries
	assetService AssetService
	pins         *pins.Service
	hmacKey      []byte
}

// NewShareLinkService constructs the share link service. secretKeyPath is the
// configured LUMILIO_SECRET_KEY file path (config.AuthConfig.SecretKeyPath);
// the token-hashing key is derived from the same root secret used for
// JWT/media-token signing (see auth_service.go's NewAuthService), under its
// own scope so a leaked share-hashing key can't be used to forge auth tokens
// and vice versa. Panics on secret key initialization failure, matching
// NewAuthService's existing convention.
func NewShareLinkService(queries *repo.Queries, assetService AssetService, pinService *pins.Service, secretKeyPath string) *shareLinkService {
	rootSecret, err := secretbox.LoadOrCreateLumilioSecretKey(strings.TrimSpace(secretKeyPath))
	if err != nil {
		panic(fmt.Sprintf("failed to initialize root secret key: %v", err))
	}
	return &shareLinkService{
		queries:      queries,
		assetService: assetService,
		pins:         pinService,
		hmacKey:      secretbox.DeriveScopedSecret(rootSecret, shareLinkTokenHashScope),
	}
}

func (s *shareLinkService) generateToken() (string, error) {
	raw := make([]byte, shareLinkTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate share token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *shareLinkService) hashToken(rawToken string) []byte {
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(rawToken))
	return mac.Sum(nil)
}

func parseInt32Ref(ref *string) (int32, error) {
	if ref == nil || strings.TrimSpace(*ref) == "" {
		return 0, fmt.Errorf("%w: source_ref is required for this source_kind", ErrShareLinkInvalidSource)
	}
	v, err := strconv.ParseInt(strings.TrimSpace(*ref), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid source_ref: %v", ErrShareLinkInvalidSource, err)
	}
	return int32(v), nil
}

// resolveSourceAssetIDs materializes a source into a concrete snapshot of
// asset IDs at creation time (the doc's "snapshot, not live query" decision).
// Every kind reuses the same authorization-scoped query paths the rest of the
// app already uses (AssetService.QueryAssets / pins.Service.AssetIDs) so a
// share can never see more than its owner already could.
func (s *shareLinkService) resolveSourceAssetIDs(ctx context.Context, ownerID int32, ownerScope *int32, kind string, ref *string, explicitIDs []uuid.UUID) ([]uuid.UUID, error) {
	switch kind {
	case "asset_snapshot":
		return s.resolveExplicitAssetIDs(ctx, ownerScope, explicitIDs)
	case "album":
		albumID, err := parseInt32Ref(ref)
		if err != nil {
			return nil, err
		}
		return s.resolveByQuery(ctx, QueryAssetsParams{OwnerID: ownerScope, AlbumID: &albumID, SortBy: "date_captured", Limit: ShareLinkMaxAssets})
	case "person":
		personID, err := parseInt32Ref(ref)
		if err != nil {
			return nil, err
		}
		return s.resolveByQuery(ctx, QueryAssetsParams{OwnerID: ownerScope, PersonID: &personID, SortBy: "date_captured", Limit: ShareLinkMaxAssets})
	case "utility_query":
		if ref == nil || strings.TrimSpace(*ref) == "" {
			return nil, fmt.Errorf("%w: utility_query source requires source_ref (tag name)", ErrShareLinkInvalidSource)
		}
		tagSource := "zeroshot"
		return s.resolveByQuery(ctx, QueryAssetsParams{OwnerID: ownerScope, TagName: ref, TagSource: &tagSource, SortBy: "date_captured", Limit: ShareLinkMaxAssets})
	case "pin":
		if ref == nil || strings.TrimSpace(*ref) == "" {
			return nil, fmt.Errorf("%w: pin source requires source_ref (pin id)", ErrShareLinkInvalidSource)
		}
		pinID, err := uuid.Parse(strings.TrimSpace(*ref))
		if err != nil {
			return nil, fmt.Errorf("%w: invalid pin source_ref: %v", ErrShareLinkInvalidSource, err)
		}
		if s.pins == nil {
			return nil, errors.New("pin sharing unavailable")
		}
		// Pins have no admin bypass today (agent refs are scoped to the
		// requesting user); always resolve against the concrete owner.
		_, ids, err := s.pins.AssetIDs(ctx, ownerID, pinID)
		if err != nil {
			return nil, err
		}
		if len(ids) > ShareLinkMaxAssets {
			return nil, ErrShareLinkTooLarge
		}
		if len(ids) == 0 {
			return nil, ErrShareLinkSourceEmpty
		}
		return ids, nil
	default:
		return nil, fmt.Errorf("%w: unsupported source_kind %q", ErrShareLinkInvalidSource, kind)
	}
}

func (s *shareLinkService) resolveByQuery(ctx context.Context, params QueryAssetsParams) ([]uuid.UUID, error) {
	assets, total, err := s.assetService.QueryAssets(ctx, params)
	if err != nil {
		return nil, err
	}
	if total > ShareLinkMaxAssets {
		return nil, ErrShareLinkTooLarge
	}
	if total == 0 {
		return nil, ErrShareLinkSourceEmpty
	}
	ids := make([]uuid.UUID, 0, len(assets))
	for _, a := range assets {
		ids = append(ids, uuid.UUID(a.AssetID.Bytes))
	}
	return ids, nil
}

func (s *shareLinkService) resolveExplicitAssetIDs(ctx context.Context, ownerScope *int32, explicitIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(explicitIDs) == 0 {
		return nil, ErrShareLinkSourceEmpty
	}
	if len(explicitIDs) > ShareLinkMaxAssets {
		return nil, ErrShareLinkTooLarge
	}

	seen := make(map[uuid.UUID]struct{}, len(explicitIDs))
	pgIDs := make([]pgtype.UUID, 0, len(explicitIDs))
	for _, id := range explicitIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		pgIDs = append(pgIDs, pgtype.UUID{Bytes: id, Valid: true})
	}

	rows, err := s.queries.GetAssetsByIDs(ctx, pgIDs)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, a := range rows {
		if ownerScope != nil && (a.OwnerID == nil || *a.OwnerID != *ownerScope) {
			continue
		}
		ids = append(ids, uuid.UUID(a.AssetID.Bytes))
	}
	if len(ids) == 0 {
		return nil, ErrShareLinkSourceEmpty
	}
	return ids, nil
}

func clampExpiryDays(days int) int {
	if days <= 0 {
		return shareLinkDefaultExpiryDays
	}
	if days > shareLinkMaxExpiryDays {
		return shareLinkMaxExpiryDays
	}
	return days
}

func (s *shareLinkService) Create(ctx context.Context, params ShareLinkCreateParams) (repo.ShareLink, string, error) {
	assetIDs, err := s.resolveSourceAssetIDs(ctx, params.OwnerID, params.OwnerScope, params.SourceKind, params.SourceRef, params.ExplicitAssetIDs)
	if err != nil {
		return repo.ShareLink{}, "", err
	}

	rawToken, err := s.generateToken()
	if err != nil {
		return repo.ShareLink{}, "", err
	}

	expiresAt := time.Now().Add(time.Duration(clampExpiryDays(params.ExpiresInDays)) * 24 * time.Hour)

	pgIDs := make([]pgtype.UUID, len(assetIDs))
	for i, id := range assetIDs {
		pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}

	link, err := s.queries.CreateShareLink(ctx, repo.CreateShareLinkParams{
		OwnerID:          params.OwnerID,
		TokenHash:        s.hashToken(rawToken),
		Title:            params.Title,
		Description:      params.Description,
		SourceKind:       params.SourceKind,
		SourceRef:        params.SourceRef,
		AssetIds:         pgIDs,
		AssetCount:       int32(len(pgIDs)),
		AllowDownload:    params.AllowDownload,
		IncludeOriginals: params.IncludeOriginals,
		ExpiresAt:        pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return repo.ShareLink{}, "", err
	}
	return link, rawToken, nil
}

func (s *shareLinkService) List(ctx context.Context, ownerID int32) ([]repo.ShareLink, error) {
	return s.queries.ListShareLinksByOwner(ctx, ownerID)
}

func (s *shareLinkService) Get(ctx context.Context, ownerID int32, shareID uuid.UUID) (repo.ShareLink, error) {
	link, err := s.queries.GetShareLinkByID(ctx, repo.GetShareLinkByIDParams{
		ShareID: pgtype.UUID{Bytes: shareID, Valid: true},
		OwnerID: ownerID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.ShareLink{}, ErrShareLinkNotFound
		}
		return repo.ShareLink{}, err
	}
	return link, nil
}

func (s *shareLinkService) UpdateSettings(ctx context.Context, ownerID int32, shareID uuid.UUID, params ShareLinkUpdateParams) (repo.ShareLink, error) {
	current, err := s.Get(ctx, ownerID, shareID)
	if err != nil {
		return repo.ShareLink{}, err
	}

	title := current.Title
	if params.Title != nil {
		title = *params.Title
	}
	description := current.Description
	if params.Description != nil {
		description = params.Description
	}
	allowDownload := current.AllowDownload
	if params.AllowDownload != nil {
		allowDownload = *params.AllowDownload
	}
	includeOriginals := current.IncludeOriginals
	if params.IncludeOriginals != nil {
		includeOriginals = *params.IncludeOriginals
	}

	updated, err := s.queries.UpdateShareLinkSettings(ctx, repo.UpdateShareLinkSettingsParams{
		ShareID:          pgtype.UUID{Bytes: shareID, Valid: true},
		OwnerID:          ownerID,
		Title:            title,
		Description:      description,
		AllowDownload:    allowDownload,
		IncludeOriginals: includeOriginals,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.ShareLink{}, ErrShareLinkNotFound
		}
		return repo.ShareLink{}, err
	}

	if params.ExtendDays != nil {
		return s.extend(ctx, ownerID, shareID, updated, *params.ExtendDays)
	}
	return updated, nil
}

// extend moves expires_at to max(now, current expiry) + days, so extending an
// already-expired link starts counting from today rather than compounding
// from a stale past expiry.
func (s *shareLinkService) extend(ctx context.Context, ownerID int32, shareID uuid.UUID, current repo.ShareLink, days int) (repo.ShareLink, error) {
	base := time.Now()
	if current.ExpiresAt.Valid && current.ExpiresAt.Time.After(base) {
		base = current.ExpiresAt.Time
	}
	newExpiry := base.Add(time.Duration(clampExpiryDays(days)) * 24 * time.Hour)

	updated, err := s.queries.ExtendShareLinkExpiry(ctx, repo.ExtendShareLinkExpiryParams{
		ShareID:   pgtype.UUID{Bytes: shareID, Valid: true},
		OwnerID:   ownerID,
		ExpiresAt: pgtype.Timestamptz{Time: newExpiry, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.ShareLink{}, ErrShareLinkNotFound
		}
		return repo.ShareLink{}, err
	}
	return updated, nil
}

func (s *shareLinkService) Revoke(ctx context.Context, ownerID int32, shareID uuid.UUID) (repo.ShareLink, error) {
	updated, err := s.queries.RevokeShareLink(ctx, repo.RevokeShareLinkParams{
		ShareID: pgtype.UUID{Bytes: shareID, Valid: true},
		OwnerID: ownerID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.ShareLink{}, ErrShareLinkNotFound
		}
		return repo.ShareLink{}, err
	}
	return updated, nil
}

func (s *shareLinkService) Delete(ctx context.Context, ownerID int32, shareID uuid.UUID) error {
	rows, err := s.queries.DeleteShareLink(ctx, repo.DeleteShareLinkParams{
		ShareID: pgtype.UUID{Bytes: shareID, Valid: true},
		OwnerID: ownerID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		// The delete WHERE clause enforces "expired or revoked only" at the SQL
		// level; distinguish "not found" from "still active" for the caller.
		if _, getErr := s.Get(ctx, ownerID, shareID); getErr == nil {
			return ErrShareLinkNotDeletable
		}
		return ErrShareLinkNotFound
	}
	return nil
}

func (s *shareLinkService) ResolvePublic(ctx context.Context, rawToken string) (repo.ShareLink, error) {
	link, err := s.queries.GetActiveShareLinkByTokenHash(ctx, s.hashToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.ShareLink{}, ErrShareLinkNotFound
		}
		return repo.ShareLink{}, err
	}
	return link, nil
}

func (s *shareLinkService) RecordView(ctx context.Context, shareID uuid.UUID) error {
	return s.queries.IncrementShareLinkView(ctx, pgtype.UUID{Bytes: shareID, Valid: true})
}

func (s *shareLinkService) PublicAssetSource(link repo.ShareLink) *AssetSetSource {
	ids := make([]uuid.UUID, len(link.AssetIds))
	for i, id := range link.AssetIds {
		ids[i] = uuid.UUID(id.Bytes)
	}
	return &AssetSetSource{
		Kind:                  AssetSetSourceShareLink,
		AssetIDs:              ids,
		PreserveSnapshotOrder: true,
	}
}

func (s *shareLinkService) AssetInShare(link repo.ShareLink, assetID uuid.UUID) bool {
	for _, id := range link.AssetIds {
		if uuid.UUID(id.Bytes) == assetID {
			return true
		}
	}
	return false
}
