package bleveocr

import (
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

type QueryMode uint8

const (
	QueryStrict QueryMode = iota
	QueryRelaxed
)

type BasicFilters struct {
	OwnerID      *int32
	RepositoryID *string
	AssetType    *string
	AssetTypes   []string
	IsDeleted    bool
}

func buildQuery(
	indexMapping mapping.IndexMapping,
	raw string,
	filters BasicFilters,
	mode QueryMode,
) (query.Query, bool) {
	textEN, textZH := SplitQuery(raw)
	textEN = analyzerNonEmptyText(indexMapping, "text_en", textEN)
	textZH = analyzerNonEmptyText(indexMapping, "text_zh", textZH)
	must := make([]query.Query, 0, 7)

	if textZH != "" {
		must = append(must, languageQuery(textZH, "text_zh", mode))
	}
	if textEN != "" {
		must = append(must, languageQuery(textEN, "text_en", mode))
	}
	if len(must) == 0 {
		return bleve.NewMatchNoneQuery(), false
	}

	deletedQuery := bleve.NewBoolFieldQuery(filters.IsDeleted)
	deletedQuery.SetField("is_deleted")
	must = append(must, deletedQuery)

	if filters.OwnerID != nil {
		owner := float64(*filters.OwnerID)
		inclusive := true
		ownerQuery := bleve.NewNumericRangeInclusiveQuery(&owner, &owner, &inclusive, &inclusive)
		ownerQuery.SetField("owner_id")
		must = append(must, ownerQuery)
	}
	if filters.RepositoryID != nil {
		must = append(must, termQuery("repository_id", strings.TrimSpace(*filters.RepositoryID)))
	}
	if filters.AssetType != nil {
		must = append(must, termQuery("asset_type", strings.TrimSpace(*filters.AssetType)))
	}
	if len(filters.AssetTypes) > 0 {
		types := make([]query.Query, 0, len(filters.AssetTypes))
		for _, assetType := range filters.AssetTypes {
			if assetType = strings.TrimSpace(assetType); assetType != "" {
				types = append(types, termQuery("asset_type", assetType))
			}
		}
		if len(types) > 0 {
			must = append(must, bleve.NewDisjunctionQuery(types...))
		}
	}

	return bleve.NewConjunctionQuery(must...), true
}

func analyzerNonEmptyText(indexMapping mapping.IndexMapping, field, text string) string {
	if text == "" {
		return ""
	}
	analyzer := indexMapping.AnalyzerNamed(indexMapping.AnalyzerNameForPath(field))
	if analyzer == nil || len(analyzer.Analyze([]byte(text))) == 0 {
		return ""
	}
	return text
}

func languageQuery(text, field string, mode QueryMode) query.Query {
	match := bleve.NewMatchQuery(text)
	match.SetField(field)
	if mode == QueryStrict {
		match.SetOperator(query.MatchQueryOperatorAnd)
	} else {
		match.SetOperator(query.MatchQueryOperatorOr)
	}
	return match
}

func termQuery(field, value string) query.Query {
	term := bleve.NewTermQuery(value)
	term.SetField(field)
	return term
}
