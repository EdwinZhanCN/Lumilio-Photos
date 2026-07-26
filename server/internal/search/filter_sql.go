package search

import (
	"encoding/json"
	"fmt"
	"strings"
)

type sqlBuilder struct {
	args []any
}

func (b *sqlBuilder) addArg(value any) string {
	b.args = append(b.args, value)
	return fmt.Sprintf("?%d", len(b.args))
}

func (b *sqlBuilder) addJSONArg(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode SQLite JSON argument: %w", err)
	}
	return b.addArg(string(encoded)), nil
}

func buildAssetFilterConditions(builder *sqlBuilder, filter Filter, assetAlias string) ([]string, error) {
	a := assetAlias
	isDeleted := false
	if filter.IsDeleted != nil {
		isDeleted = *filter.IsDeleted
	}
	conditions := []string{fmt.Sprintf("%s.is_deleted = %s", a, builder.addArg(isDeleted))}

	if filter.AssetIDs != nil {
		placeholder, err := builder.addJSONArg(filter.AssetIDs)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, fmt.Sprintf("%s.asset_id IN (SELECT value FROM json_each(%s))", a, placeholder))
	}
	if filter.AssetType != nil {
		conditions = append(conditions, fmt.Sprintf("%s.type = %s", a, builder.addArg(*filter.AssetType)))
	}
	if len(filter.AssetTypes) > 0 {
		placeholder, err := builder.addJSONArg(filter.AssetTypes)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, fmt.Sprintf("%s.type IN (SELECT value FROM json_each(%s))", a, placeholder))
	}
	if filter.OwnerID != nil {
		conditions = append(conditions, fmt.Sprintf("%s.owner_id = %s", a, builder.addArg(*filter.OwnerID)))
	}
	if filter.RepositoryID != nil {
		conditions = append(conditions, fmt.Sprintf("%s.repository_id = %s", a, builder.addArg(*filter.RepositoryID)))
	}
	if filter.PersonID != nil {
		personPlaceholder := builder.addArg(*filter.PersonID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM face_cluster_members fcm
			JOIN face_items fi_person ON fi_person.id = fcm.face_id
			WHERE fcm.cluster_id = %s
			  AND fi_person.asset_id = %s.asset_id
		)`, personPlaceholder, a))
	}
	if filter.AlbumID != nil {
		albumPlaceholder := builder.addArg(*filter.AlbumID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM album_assets aa
			WHERE aa.asset_id = %s.asset_id
			  AND aa.album_id = %s
		)`, a, albumPlaceholder))
	}
	if filter.TagName != nil {
		tagNamePlaceholder := builder.addArg(*filter.TagName)
		tagSourceCondition := ""
		if filter.TagSource != nil {
			tagSourcePlaceholder := builder.addArg(*filter.TagSource)
			tagSourceCondition = fmt.Sprintf("\n			  AND at.source = %s", tagSourcePlaceholder)
		}
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM asset_tags at
			JOIN tags t ON t.tag_id = at.tag_id
			WHERE at.asset_id = %s.asset_id
			  AND t.tag_name = %s%s
		)`, a, tagNamePlaceholder, tagSourceCondition))
	}
	if len(filter.TagNames) > 0 {
		tagNamesPlaceholder, err := builder.addJSONArg(filter.TagNames)
		if err != nil {
			return nil, err
		}
		// Match assets carrying every requested tag (AND semantics).
		conditions = append(conditions, fmt.Sprintf(`(
			SELECT COUNT(DISTINCT t.tag_name)
			FROM asset_tags at
			JOIN tags t ON t.tag_id = at.tag_id
			WHERE at.asset_id = %s.asset_id
			  AND t.tag_name IN (SELECT value FROM json_each(%s))
		) = json_array_length(%s)`, a, tagNamesPlaceholder, tagNamesPlaceholder))
	}
	if filter.FilenameValue != nil {
		filenamePlaceholder := builder.addArg(*filter.FilenameValue)
		switch {
		case filter.FilenameOperator != nil && *filter.FilenameOperator == "matches":
			conditions = append(conditions, fmt.Sprintf("%s.original_filename LIKE %s", a, filenamePlaceholder))
		case filter.FilenameOperator != nil && *filter.FilenameOperator == "starts_with":
			conditions = append(conditions, fmt.Sprintf("%s.original_filename LIKE %s || '%%'", a, filenamePlaceholder))
		case filter.FilenameOperator != nil && *filter.FilenameOperator == "ends_with":
			conditions = append(conditions, fmt.Sprintf("%s.original_filename LIKE '%%' || %s", a, filenamePlaceholder))
		default:
			conditions = append(conditions, fmt.Sprintf("%s.original_filename LIKE '%%' || %s || '%%'", a, filenamePlaceholder))
		}
	}
	if filter.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.taken_time, %s.upload_time) >= %s", a, a, builder.addArg(filter.DateFrom.UTC().UnixMicro())))
	}
	if filter.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.taken_time, %s.upload_time) <= %s", a, a, builder.addArg(filter.DateTo.UTC().UnixMicro())))
	}
	if filter.IsRaw != nil {
		if *filter.IsRaw {
			conditions = append(conditions, fmt.Sprintf("json_extract(%s.specific_metadata, char(36) || '.is_raw') = 1", a))
		} else {
			conditions = append(conditions, fmt.Sprintf("COALESCE(json_extract(%s.specific_metadata, char(36) || '.is_raw'), 0) = 0", a))
		}
	}
	if filter.Rating != nil {
		if *filter.Rating == 0 {
			conditions = append(conditions, fmt.Sprintf("(%s.rating IS NULL OR %s.rating = 0)", a, a))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s.rating = %s", a, builder.addArg(*filter.Rating)))
		}
	}
	if filter.Liked != nil {
		if *filter.Liked {
			conditions = append(conditions, a+".liked = true")
		} else {
			conditions = append(conditions, fmt.Sprintf("(%s.liked IS NULL OR %s.liked = false)", a, a))
		}
	}
	if filter.CameraModel != nil {
		conditions = append(conditions, fmt.Sprintf("json_extract(%s.specific_metadata, char(36) || '.camera_model') = %s", a, builder.addArg(*filter.CameraModel)))
	}
	if filter.LensModel != nil {
		conditions = append(conditions, fmt.Sprintf("json_extract(%s.specific_metadata, char(36) || '.lens_model') = %s", a, builder.addArg(*filter.LensModel)))
	}
	if filter.LocationNorth != nil && filter.LocationSouth != nil && filter.LocationEast != nil && filter.LocationWest != nil {
		northPlaceholder := builder.addArg(*filter.LocationNorth)
		southPlaceholder := builder.addArg(*filter.LocationSouth)
		eastPlaceholder := builder.addArg(*filter.LocationEast)
		westPlaceholder := builder.addArg(*filter.LocationWest)
		conditions = append(conditions, fmt.Sprintf(`%s.gps_latitude IS NOT NULL
  AND %s.gps_longitude IS NOT NULL
  AND %s.gps_latitude BETWEEN min(%s, %s) AND max(%s, %s)
  AND (
    CASE
      WHEN %s <= %s THEN %s.gps_longitude BETWEEN %s AND %s
      ELSE %s.gps_longitude >= %s OR %s.gps_longitude <= %s
    END
  )`, a, a, a, southPlaceholder, northPlaceholder, southPlaceholder, northPlaceholder, westPlaceholder, eastPlaceholder, a, westPlaceholder, eastPlaceholder, a, westPlaceholder, a, eastPlaceholder))
	}

	return conditions, nil
}

func joinConditions(conditions []string) string {
	return strings.Join(conditions, "\n  AND ")
}
