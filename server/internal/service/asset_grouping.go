package service

import (
	"fmt"
	"strings"
	"time"

	"server/internal/db/repo"
)

type AssetGroup struct {
	Key    string
	Assets []repo.Asset
}

func GroupAssetsPage(
	assets []repo.Asset,
	groupBy string,
	viewerTimeZone string,
) []AssetGroup {
	return groupAssetsPageAt(assets, normalizeAssetGroupBy(groupBy), viewerTimeZone, time.Now())
}

func groupAssetsPageAt(
	assets []repo.Asset,
	groupBy string,
	viewerTimeZone string,
	now time.Time,
) []AssetGroup {
	if len(assets) == 0 {
		return []AssetGroup{}
	}

	location := resolveViewerLocation(viewerTimeZone)
	nowLocal := now.In(location)
	groups := make([]AssetGroup, 0, len(assets))

	for _, asset := range assets {
		key := buildAssetGroupKey(asset, groupBy, location, nowLocal)
		if len(groups) == 0 || groups[len(groups)-1].Key != key {
			groups = append(groups, AssetGroup{
				Key:    key,
				Assets: []repo.Asset{asset},
			})
			continue
		}

		last := &groups[len(groups)-1]
		last.Assets = append(last.Assets, asset)
	}

	return groups
}

func normalizeAssetGroupBy(groupBy string) string {
	switch strings.TrimSpace(groupBy) {
	case "date":
		return "date"
	case "type":
		return "type"
	default:
		return "flat"
	}
}

func resolveViewerLocation(viewerTimeZone string) *time.Location {
	if strings.TrimSpace(viewerTimeZone) == "" {
		return time.UTC
	}

	location, err := time.LoadLocation(strings.TrimSpace(viewerTimeZone))
	if err != nil {
		return time.UTC
	}

	return location
}

func buildAssetGroupKey(
	asset repo.Asset,
	groupBy string,
	location *time.Location,
	nowLocal time.Time,
) string {
	switch groupBy {
	case "date":
		return buildDateGroupKey(assetTime(asset).In(location), nowLocal)
	case "type":
		return buildTypeGroupKey(asset)
	default:
		return "flat:all"
	}
}

func assetTime(asset repo.Asset) time.Time {
	if asset.TakenTime.Valid {
		return asset.TakenTime.Time
	}
	if asset.UploadTime.Valid {
		return asset.UploadTime.Time
	}
	return time.Unix(0, 0).UTC()
}

func buildDateGroupKey(assetTime time.Time, nowLocal time.Time) string {
	assetDate := time.Date(
		assetTime.Year(),
		assetTime.Month(),
		assetTime.Day(),
		0,
		0,
		0,
		0,
		assetTime.Location(),
	)
	today := time.Date(
		nowLocal.Year(),
		nowLocal.Month(),
		nowLocal.Day(),
		0,
		0,
		0,
		0,
		nowLocal.Location(),
	)
	yesterday := today.AddDate(0, 0, -1)
	thisWeekStart := today.AddDate(0, 0, -int(today.Weekday()))
	thisMonthStart := time.Date(nowLocal.Year(), nowLocal.Month(), 1, 0, 0, 0, 0, nowLocal.Location())
	thisYearStart := time.Date(nowLocal.Year(), 1, 1, 0, 0, 0, 0, nowLocal.Location())

	switch {
	case assetDate.Equal(today):
		return "date:today"
	case assetDate.Equal(yesterday):
		return "date:yesterday"
	case !assetDate.Before(thisWeekStart):
		return "date:this_week"
	case !assetDate.Before(thisMonthStart):
		return "date:this_month"
	case !assetDate.Before(thisYearStart):
		return fmt.Sprintf("date:month:%04d-%02d", assetDate.Year(), int(assetDate.Month()))
	default:
		return fmt.Sprintf("date:year:%04d", assetDate.Year())
	}
}

func buildTypeGroupKey(asset repo.Asset) string {
	mime := strings.TrimSpace(asset.MimeType)
	if mime != "" {
		return "type:" + mime
	}

	switch strings.ToUpper(strings.TrimSpace(asset.Type)) {
	case AssetTypePhoto:
		return "type:image/*"
	case AssetTypeVideo:
		return "type:video/*"
	case AssetTypeAudio:
		return "type:audio/*"
	default:
		return "type:unknown"
	}
}
