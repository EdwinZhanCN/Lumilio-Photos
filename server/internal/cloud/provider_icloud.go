package cloud

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"

	"server/internal/cloud/icloud"
)

// ICloudConfig holds the configuration for creating an iCloud provider.
type ICloudConfig struct {
	// Username is the Apple ID email address.
	Username string

	// Password is an app-specific password for authentication.
	Password string

	// CookieDir is the directory where session cookies are persisted.
	// Defaults to {STORAGE_PATH}/.icloud/ when empty.
	CookieDir string

	// Domain is "com" (default) or "cn" for mainland China.
	Domain string
}

// ICloudProvider implements CloudProvider for Apple iCloud Photos using
// the native Go iCloud client (no external CLI dependency).
type ICloudProvider struct {
	config          ICloudConfig
	client          *icloud.Client
	photoCli        *icloud.PhotoService
	cookieDir       string
	assetCache      sync.Map
	twoFACodeGetter icloud.TextGetter
}

// NewICloudProvider creates an iCloud provider from the given configuration.
func NewICloudProvider(cfg ICloudConfig) *ICloudProvider {
	if cfg.Domain == "" {
		cfg.Domain = "com"
	}
	return &ICloudProvider{config: cfg}
}

// Name returns the provider identifier.
func (p *ICloudProvider) Name() ProviderKind { return ProviderICloud }

// ensureClient initializes the icloud.Client and authenticates if needed.
func (p *ICloudProvider) ensureClient(ctx context.Context) error {
	_ = ctx
	if p.client != nil {
		return nil
	}

	cookieDir := p.config.CookieDir
	if cookieDir == "" {
		cookieDir = defaultICloudCookieDir()
	}
	p.cookieDir = cookieDir

	client, err := icloud.NewClient(&icloud.ClientOption{
		AppID:           p.config.Username,
		Password:        p.config.Password,
		CookieDir:       cookieDir,
		Domain:          p.config.Domain,
		TwoFACodeGetter: p.twoFACodeGetter,
	})
	if err != nil {
		return fmt.Errorf("create icloud client: %w", err)
	}

	// Authenticate (blocks on MFA if needed via TwoFACodeGetter)
	if err := client.Authenticate(false, nil); err != nil {
		return fmt.Errorf("icloud authenticate: %w", err)
	}

	p.client = client
	return nil
}

// ensurePhotoCli initializes the PhotoService lazily.
func (p *ICloudProvider) ensurePhotoCli(ctx context.Context) (*icloud.PhotoService, error) {
	if err := p.ensureClient(ctx); err != nil {
		return nil, err
	}
	if p.photoCli == nil {
		cli, err := p.client.PhotoCli()
		if err != nil {
			return nil, fmt.Errorf("icloud photo service: %w", err)
		}
		p.photoCli = cli
	}
	return p.photoCli, nil
}

// SetTwoFACodeGetter sets the MFA code provider. Must be called before
// any List/Download operation if MFA is required.
func (p *ICloudProvider) SetTwoFACodeGetter(getter icloud.TextGetter) {
	p.twoFACodeGetter = getter
	if p.client != nil {
		p.client.SetTwoFACodeGetter(getter)
	}
}

// ForceAuth explicitly triggers authentication. This is useful when the caller
// needs to control the auth lifecycle (e.g., for 2FA flows where the code
// getter is set between calls).
func (p *ICloudProvider) ForceAuth(ctx context.Context) error {
	return p.ensureClient(ctx)
}

// IsAuthenticated reports whether the provider has a valid session.
func (p *ICloudProvider) IsAuthenticated() bool {
	if p.client == nil {
		return false
	}
	return p.client.Data != nil && p.client.Data.DsInfo != nil
}

// List retrieves all photos from iCloud "All Photos" album in a paginated way
// using the provided Cursor.
func (p *ICloudProvider) List(ctx context.Context, repoID uuid.UUID, cursor *Cursor) (*Page, error) {
	_ = repoID

	photoCli, err := p.ensurePhotoCli(ctx)
	if err != nil {
		return nil, err
	}

	album, err := photoCli.GetAlbum(icloud.AlbumNameAll)
	if err != nil {
		return nil, fmt.Errorf("get all photos album: %w", err)
	}

	offset := int64(0)
	if cursor != nil && cursor.Value != "" {
		parsed, err := strconv.ParseInt(cursor.Value, 10, 64)
		if err == nil {
			offset = parsed
		}
	} else {
		if album.Direction == "DESCENDING" {
			offset = album.Size() - 1
		}
	}

	const batchSize = 200
	photos, err := album.GetPhotosByOffset(offset, batchSize)
	if err != nil {
		return nil, fmt.Errorf("get photos by offset %d: %w", offset, err)
	}

	var assets []ReleaseAsset
	for _, photo := range photos {
		// Cache the photo asset pointer for O(1) downloads
		p.assetCache.Store(photo.ID(), photo)

		assets = append(assets, ReleaseAsset{
			Provider:   ProviderICloud,
			RemoteKey:  photo.ID(),
			Filename:   photo.Filename(false),
			Size:       int64(photo.Size()),
			MIME:       photo.MIMEType(),
			ETag:       photo.Fingerprint(),
			ModifiedAt: photo.AddDate(),
			Deleted:    photo.IsDeleted(),
		})
	}

	var newOffset int64
	if album.Direction == "DESCENDING" {
		newOffset = offset - int64(len(photos))
	} else {
		newOffset = offset + int64(len(photos))
	}
	hasMore := len(photos) == batchSize

	var nextCursor *Cursor
	if hasMore {
		nextCursor = &Cursor{
			Value: strconv.FormatInt(newOffset, 10),
		}
	}

	return &Page{
		Assets:  assets,
		Cursor:  nextCursor,
		HasMore: hasMore,
	}, nil
}

// Download fetches a specific iCloud photo asset by its remoteKey (asset ID)
// and writes it to localPath. It checks the local cache first, and falls back
// to walking the album if cache misses.
func (p *ICloudProvider) Download(ctx context.Context, repoID uuid.UUID, remoteKey string, localPath string) (int64, error) {
	_ = repoID

	photoCli, err := p.ensurePhotoCli(ctx)
	if err != nil {
		return 0, err
	}

	album, err := photoCli.GetAlbum(icloud.AlbumNameAll)
	if err != nil {
		return 0, fmt.Errorf("get all photos album: %w", err)
	}

	// Try to get from cache first
	var target *icloud.PhotoAsset
	if val, ok := p.assetCache.Load(remoteKey); ok {
		target = val.(*icloud.PhotoAsset)
	} else {
		// Fallback to walk if not in cache (e.g. if single file download triggered manually)
		_ = album.WalkPhotos(0, func(offset int64, photos []*icloud.PhotoAsset) error {
			for _, photo := range photos {
				if photo.ID() == remoteKey {
					target = photo
					return fmt.Errorf("found") // break the walk
				}
			}
			return nil
		})
	}

	if target == nil {
		return 0, fmt.Errorf("photo %s not found in iCloud", remoteKey)
	}

	// Download to the target path
	if err := target.DownloadTo(icloud.PhotoVersionOriginal, false, localPath); err != nil {
		return 0, fmt.Errorf("download photo %s: %w", remoteKey, err)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return 0, fmt.Errorf("stat downloaded file: %w", err)
	}

	return info.Size(), nil
}

// defaultICloudCookieDir resolves the default icloud cookie directory
// using the same path resolution as .secrets.
func defaultICloudCookieDir() string {
	storagePath := strings.TrimSpace(os.Getenv("STORAGE_PATH"))
	if storagePath != "" {
		normalized := filepath.Clean(storagePath)
		if strings.EqualFold(filepath.Base(normalized), "primary") {
			normalized = filepath.Dir(normalized)
		}
		return filepath.Join(normalized, ".icloud")
	}
	return filepath.Join("data", "storage", ".icloud")
}
