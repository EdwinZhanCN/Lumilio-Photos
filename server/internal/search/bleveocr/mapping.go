package bleveocr

import (
	"fmt"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/lang/cjk"
	"github.com/blevesearch/bleve/v2/analysis/lang/en"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/mapping"
)

const (
	MappingVersion = "1"
	zhAnalyzerName = "lumilio_zh"
	zhBigramName   = "lumilio_cjk_bigram"
)

var mappingVersionKey = []byte("lumilio.ocr.mapping_version")

func NewMapping() (*mapping.IndexMappingImpl, error) {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.IndexDynamic = false
	indexMapping.StoreDynamic = false
	indexMapping.DocValuesDynamic = false

	if err := indexMapping.AddCustomTokenFilter(zhBigramName, map[string]any{
		"type":           cjk.BigramName,
		"output_unigram": false,
	}); err != nil {
		return nil, fmt.Errorf("register OCR CJK bigram filter: %w", err)
	}
	if err := indexMapping.AddCustomAnalyzer(zhAnalyzerName, map[string]any{
		"type":      custom.Name,
		"tokenizer": unicode.Name,
		"token_filters": []string{
			cjk.WidthName,
			zhBigramName,
			lowercase.Name,
		},
	}); err != nil {
		return nil, fmt.Errorf("register OCR Chinese analyzer: %w", err)
	}

	documentMapping := bleve.NewDocumentStaticMapping()
	documentMapping.AddFieldMappingsAt("asset_id", keywordField())
	documentMapping.AddFieldMappingsAt("text_en", textField(en.AnalyzerName))
	documentMapping.AddFieldMappingsAt("text_zh", textField(zhAnalyzerName))
	documentMapping.AddFieldMappingsAt("owner_id", numericField())
	documentMapping.AddFieldMappingsAt("repository_id", keywordField())
	documentMapping.AddFieldMappingsAt("asset_type", keywordField())
	documentMapping.AddFieldMappingsAt("is_deleted", booleanField())
	documentMapping.AddFieldMappingsAt("revision", numericField())
	indexMapping.DefaultMapping = documentMapping

	return indexMapping, nil
}

func textField(analyzer string) *mapping.FieldMapping {
	field := bleve.NewTextFieldMapping()
	field.Analyzer = analyzer
	field.Store = false
	field.IncludeTermVectors = false
	field.IncludeInAll = false
	field.DocValues = false
	return field
}

func keywordField() *mapping.FieldMapping {
	field := bleve.NewKeywordFieldMapping()
	field.Store = false
	field.IncludeTermVectors = false
	field.IncludeInAll = false
	field.DocValues = false
	return field
}

func numericField() *mapping.FieldMapping {
	field := bleve.NewNumericFieldMapping()
	field.Store = false
	field.IncludeInAll = false
	field.DocValues = false
	return field
}

func booleanField() *mapping.FieldMapping {
	field := bleve.NewBooleanFieldMapping()
	field.Store = false
	field.IncludeInAll = false
	field.DocValues = false
	return field
}
