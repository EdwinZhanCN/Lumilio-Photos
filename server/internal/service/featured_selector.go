package service

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sort"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

const (
	defaultFeaturedCount        = 8
	defaultFeaturedMaxPerDay    = 2
	defaultFeaturedMaxPerCamera = 3
	defaultFeaturedMaxPerAspect = 4
)

// FeaturedSelectionOptions configures deterministic weighted selection.
type FeaturedSelectionOptions struct {
	Count        int
	Seed         string
	Now          time.Time
	MaxPerDay    int
	MaxPerCamera int
	MaxPerAspect int
}

type featuredCandidate struct {
	asset        repo.Asset
	assetID      string
	weight       float64
	aesKey       float64
	dayBucket    string
	cameraBucket string
	aspectBucket string
}

// SelectFeaturedPhotos chooses a small featured subset using:
// 1) quality/recency weighting
// 2) deterministic A-ES weighted sampling key
// 3) diversity constraints with safe fallback filling.
func SelectFeaturedPhotos(candidates []repo.Asset, options FeaturedSelectionOptions) []repo.Asset {
	if len(candidates) == 0 {
		return []repo.Asset{}
	}

	count := options.Count
	if count <= 0 {
		count = defaultFeaturedCount
	}
	if count > len(candidates) {
		count = len(candidates)
	}

	maxPerDay := options.MaxPerDay
	if maxPerDay <= 0 {
		maxPerDay = defaultFeaturedMaxPerDay
	}

	maxPerCamera := options.MaxPerCamera
	if maxPerCamera <= 0 {
		maxPerCamera = defaultFeaturedMaxPerCamera
	}

	maxPerAspect := options.MaxPerAspect
	if maxPerAspect <= 0 {
		maxPerAspect = defaultFeaturedMaxPerAspect
	}

	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	seed := strings.TrimSpace(options.Seed)
	if seed == "" {
		seed = now.Format("2006-01-02")
	}

	uniq := make([]repo.Asset, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, asset := range candidates {
		id := assetUUIDString(asset)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, asset)
	}

	if len(uniq) == 0 {
		return []repo.Asset{}
	}
	if count > len(uniq) {
		count = len(uniq)
	}

	ranked := make([]featuredCandidate, 0, len(uniq))
	for _, asset := range uniq {
		id := assetUUIDString(asset)
		if id == "" {
			continue
		}

		meta, _ := decodePhotoMetadata(asset)
		weight := computeFeatureWeight(asset, meta, now)
		u := deterministicUnit(seed, id)
		aesKey := -math.Log(u) / weight

		ranked = append(ranked, featuredCandidate{
			asset:        asset,
			assetID:      id,
			weight:       weight,
			aesKey:       aesKey,
			dayBucket:    buildDayBucket(asset, meta),
			cameraBucket: buildCameraBucket(meta),
			aspectBucket: buildAspectBucket(asset),
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].aesKey == ranked[j].aesKey {
			return ranked[i].assetID < ranked[j].assetID
		}
		return ranked[i].aesKey < ranked[j].aesKey
	})

	selected := make([]repo.Asset, 0, count)
	selectedID := make(map[string]struct{}, count)
	dayCount := map[string]int{}
	cameraCount := map[string]int{}
	aspectCount := map[string]int{}

	for _, candidate := range ranked {
		if len(selected) >= count {
			break
		}
		if _, ok := selectedID[candidate.assetID]; ok {
			continue
		}
		if candidate.dayBucket != "" && dayCount[candidate.dayBucket] >= maxPerDay {
			continue
		}
		if candidate.cameraBucket != "" && cameraCount[candidate.cameraBucket] >= maxPerCamera {
			continue
		}
		if candidate.aspectBucket != "" && aspectCount[candidate.aspectBucket] >= maxPerAspect {
			continue
		}

		selected = append(selected, candidate.asset)
		selectedID[candidate.assetID] = struct{}{}
		if candidate.dayBucket != "" {
			dayCount[candidate.dayBucket]++
		}
		if candidate.cameraBucket != "" {
			cameraCount[candidate.cameraBucket]++
		}
		if candidate.aspectBucket != "" {
			aspectCount[candidate.aspectBucket]++
		}
	}

	// Fallback pass: if constraints are too tight, fill remaining slots by rank.
	if len(selected) < count {
		for _, candidate := range ranked {
			if len(selected) >= count {
				break
			}
			if _, ok := selectedID[candidate.assetID]; ok {
				continue
			}
			selected = append(selected, candidate.asset)
			selectedID[candidate.assetID] = struct{}{}
		}
	}

	return selected
}

func computeFeatureWeight(
	asset repo.Asset,
	meta dbtypes.PhotoSpecificMetadata,
	now time.Time,
) float64 {
	const minWeight = 0.05

	taken := resolveTakenTime(asset, meta)
	if taken.IsZero() {
		taken = now
	}

	daysOld := now.Sub(taken).Hours() / 24
	if daysOld < 0 {
		daysOld = 0
	}

	// Half-life recency: ~120 days.
	recency := math.Exp(-math.Ln2 * daysOld / 120.0)

	resolution := 0.0
	if asset.Width != nil && asset.Height != nil && *asset.Width > 0 && *asset.Height > 0 {
		mp := float64(*asset.Width) * float64(*asset.Height) / 1_000_000.0
		resolution = clamp01(mp / 24.0)
	}

	ratingNorm := 0.0
	if asset.Rating != nil && *asset.Rating > 0 {
		ratingNorm = clamp01(float64(*asset.Rating) / 5.0)
	}

	likedNorm := 0.0
	if asset.Liked != nil && *asset.Liked {
		likedNorm = 1.0
	}

	metadataRichness := 0.0
	if meta.TakenTime != nil {
		metadataRichness += 0.20
	}
	if strings.TrimSpace(meta.CameraModel) != "" {
		metadataRichness += 0.20
	}
	if strings.TrimSpace(meta.LensModel) != "" {
		metadataRichness += 0.15
	}
	if hasValidGPS(meta) {
		metadataRichness += 0.20
	}
	if strings.TrimSpace(meta.ExposureTime) != "" {
		metadataRichness += 0.10
	}
	if meta.FNumber > 0 {
		metadataRichness += 0.15
	}
	metadataRichness = clamp01(metadataRichness)

	quality := 0.45*ratingNorm + 0.20*likedNorm + 0.35*resolution
	score := 0.45*recency + 0.35*quality + 0.20*metadataRichness

	weight := minWeight + 0.95*clamp01(score)
	if !isFinite(weight) {
		return minWeight
	}
	return math.Max(weight, minWeight)
}

func deterministicUnit(seed, assetID string) float64 {
	sum := sha256.Sum256([]byte(seed + ":" + assetID))
	v := binary.BigEndian.Uint64(sum[:8])

	// Map [0, MaxUint64] -> (0,1) to keep log stable.
	u := float64(v) / float64(^uint64(0))
	if u <= 0 {
		return 1e-12
	}
	if u >= 1 {
		return 1 - 1e-12
	}
	return u
}

func resolveTakenTime(asset repo.Asset, meta dbtypes.PhotoSpecificMetadata) time.Time {
	if meta.TakenTime != nil {
		return meta.TakenTime.UTC()
	}
	if asset.TakenTime.Valid {
		return asset.TakenTime.Time.UTC()
	}
	if asset.UploadTime.Valid {
		return asset.UploadTime.Time.UTC()
	}
	return time.Time{}
}

func buildDayBucket(asset repo.Asset, meta dbtypes.PhotoSpecificMetadata) string {
	t := resolveTakenTime(asset, meta)
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func buildCameraBucket(meta dbtypes.PhotoSpecificMetadata) string {
	camera := strings.TrimSpace(meta.CameraModel)
	if camera == "" {
		return ""
	}
	return strings.ToLower(camera)
}

func buildAspectBucket(asset repo.Asset) string {
	if asset.Width == nil || asset.Height == nil || *asset.Width <= 0 || *asset.Height <= 0 {
		return ""
	}
	ratio := float64(*asset.Width) / float64(*asset.Height)
	switch {
	case ratio >= 1.2:
		return "landscape"
	case ratio <= 0.85:
		return "portrait"
	default:
		return "squareish"
	}
}

func decodePhotoMetadata(asset repo.Asset) (dbtypes.PhotoSpecificMetadata, bool) {
	if strings.ToUpper(asset.Type) != string(dbtypes.AssetTypePhoto) {
		return dbtypes.PhotoSpecificMetadata{}, false
	}
	if len(asset.SpecificMetadata) == 0 {
		return dbtypes.PhotoSpecificMetadata{}, false
	}

	meta, err := asset.SpecificMetadata.UnmarshalPhoto()
	if err != nil {
		return dbtypes.PhotoSpecificMetadata{}, false
	}
	return meta, true
}

func assetUUIDString(asset repo.Asset) string {
	if !asset.AssetID.Valid {
		return ""
	}
	id, err := uuid.FromBytes(asset.AssetID.Bytes[:])
	if err != nil {
		return ""
	}
	return id.String()
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func hasValidGPS(meta dbtypes.PhotoSpecificMetadata) bool {
	if !isFinite(meta.GPSLatitude) || !isFinite(meta.GPSLongitude) {
		return false
	}
	// Treat (0,0) as unknown/noisy metadata; keep equator/prime-meridian valid otherwise.
	return !(meta.GPSLatitude == 0 && meta.GPSLongitude == 0)
}
