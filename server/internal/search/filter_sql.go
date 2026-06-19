package search

import (
	"fmt"
	"strings"
)

type sqlBuilder struct {
	args []any
}

func (b *sqlBuilder) addArg(value any) string {
	b.args = append(b.args, value)
	return fmt.Sprintf("$%d", len(b.args))
}

func buildAssetFilterConditions(builder *sqlBuilder, filter Filter, assetAlias string) ([]string, error) {
	a := assetAlias
	isDeleted := false
	if filter.IsDeleted != nil {
		isDeleted = *filter.IsDeleted
	}
	conditions := []string{fmt.Sprintf("%s.is_deleted = %s", a, builder.addArg(isDeleted))}

	if filter.AssetType != nil {
		conditions = append(conditions, fmt.Sprintf("%s.type = %s", a, builder.addArg(*filter.AssetType)))
	}
	if len(filter.AssetTypes) > 0 {
		conditions = append(conditions, fmt.Sprintf("%s.type = ANY(%s::text[])", a, builder.addArg(filter.AssetTypes)))
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
		tagNamesPlaceholder := builder.addArg(filter.TagNames)
		// Match assets carrying every requested tag (AND semantics).
		conditions = append(conditions, fmt.Sprintf(`(
			SELECT COUNT(DISTINCT t.tag_name)
			FROM asset_tags at
			JOIN tags t ON t.tag_id = at.tag_id
			WHERE at.asset_id = %s.asset_id
			  AND t.tag_name = ANY(%s::text[])
		) = cardinality(%s::text[])`, a, tagNamesPlaceholder, tagNamesPlaceholder))
	}
	if filter.FilenameValue != nil {
		filenamePlaceholder := builder.addArg(*filter.FilenameValue)
		switch {
		case filter.FilenameOperator != nil && *filter.FilenameOperator == "matches":
			conditions = append(conditions, fmt.Sprintf("%s.original_filename ILIKE %s", a, filenamePlaceholder))
		case filter.FilenameOperator != nil && *filter.FilenameOperator == "starts_with":
			conditions = append(conditions, fmt.Sprintf("%s.original_filename ILIKE %s || '%%'", a, filenamePlaceholder))
		case filter.FilenameOperator != nil && *filter.FilenameOperator == "ends_with":
			conditions = append(conditions, fmt.Sprintf("%s.original_filename ILIKE '%%' || %s", a, filenamePlaceholder))
		default:
			conditions = append(conditions, fmt.Sprintf("%s.original_filename ILIKE '%%' || %s || '%%'", a, filenamePlaceholder))
		}
	}
	if filter.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.taken_time, %s.upload_time) >= %s", a, a, builder.addArg(*filter.DateFrom)))
	}
	if filter.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("COALESCE(%s.taken_time, %s.upload_time) <= %s", a, a, builder.addArg(*filter.DateTo)))
	}
	if filter.IsRaw != nil {
		if *filter.IsRaw {
			conditions = append(conditions, fmt.Sprintf("%s.specific_metadata->>'is_raw' = 'true'", a))
		} else {
			conditions = append(conditions, fmt.Sprintf("(%s.specific_metadata->>'is_raw' = 'false' OR %s.specific_metadata->>'is_raw' IS NULL)", a, a))
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
		conditions = append(conditions, fmt.Sprintf("%s.specific_metadata->>'camera_model' = %s", a, builder.addArg(*filter.CameraModel)))
	}
	if filter.LensModel != nil {
		conditions = append(conditions, fmt.Sprintf("%s.specific_metadata->>'lens_model' = %s", a, builder.addArg(*filter.LensModel)))
	}
	if filter.LocationNorth != nil && filter.LocationSouth != nil && filter.LocationEast != nil && filter.LocationWest != nil {
		northPlaceholder := builder.addArg(*filter.LocationNorth)
		southPlaceholder := builder.addArg(*filter.LocationSouth)
		eastPlaceholder := builder.addArg(*filter.LocationEast)
		westPlaceholder := builder.addArg(*filter.LocationWest)
		conditions = append(conditions, fmt.Sprintf(`%s.gps_latitude IS NOT NULL
  AND %s.gps_longitude IS NOT NULL
  AND %s.gps_latitude BETWEEN LEAST(%s::float8, %s::float8) AND GREATEST(%s::float8, %s::float8)
  AND (
    CASE
      WHEN %s::float8 <= %s::float8 THEN %s.gps_longitude BETWEEN %s::float8 AND %s::float8
      ELSE %s.gps_longitude >= %s::float8 OR %s.gps_longitude <= %s::float8
    END
  )`, a, a, a, southPlaceholder, northPlaceholder, southPlaceholder, northPlaceholder, westPlaceholder, eastPlaceholder, a, westPlaceholder, eastPlaceholder, a, westPlaceholder, a, eastPlaceholder))
	}

	return conditions, nil
}

func joinConditions(conditions []string) string {
	return strings.Join(conditions, "\n  AND ")
}
