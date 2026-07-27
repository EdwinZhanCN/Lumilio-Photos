package bleveocr

import (
	"strings"
	"unicode"
)

type OCRDocument struct {
	AssetID      string `json:"asset_id"`
	TextEN       string `json:"text_en"`
	TextZH       string `json:"text_zh"`
	OwnerID      int32  `json:"owner_id"`
	RepositoryID string `json:"repository_id"`
	AssetType    string `json:"asset_type"`
	IsDeleted    bool   `json:"is_deleted"`
	Revision     int64  `json:"revision"`
}

type SourceDocument struct {
	AssetID      string
	OwnerID      int32
	RepositoryID string
	AssetType    string
	IsDeleted    bool
	Revision     int64
	TextItems    []string
}

func BuildDocument(source SourceDocument) OCRDocument {
	textEN, textZH := SplitText(strings.Join(source.TextItems, " "))
	return OCRDocument{
		AssetID:      source.AssetID,
		TextEN:       textEN,
		TextZH:       textZH,
		OwnerID:      source.OwnerID,
		RepositoryID: source.RepositoryID,
		AssetType:    source.AssetType,
		IsDeleted:    source.IsDeleted,
		Revision:     source.Revision,
	}
}

func (d OCRDocument) HasSearchableText() bool {
	return strings.TrimSpace(d.TextEN) != "" || strings.TrimSpace(d.TextZH) != ""
}

func SplitText(text string) (textEN, textZH string) {
	return splitText(text, true)
}

func SplitQuery(text string) (textEN, textZH string) {
	return splitText(text, true)
}

type scriptClass uint8

const (
	scriptSeparator scriptClass = iota
	scriptHan
	scriptLatinNumber
)

func splitText(text string, dropSingleHan bool) (textEN, textZH string) {
	enTokens := make([]string, 0)
	zhTokens := make([]string, 0)
	run := make([]rune, 0)
	class := scriptSeparator
	runHasDigit := false

	flush := func() {
		if len(run) == 0 {
			return
		}
		token := string(run)
		switch class {
		case scriptHan:
			if !dropSingleHan || len(run) > 1 {
				zhTokens = append(zhTokens, token)
			}
		case scriptLatinNumber:
			enTokens = append(enTokens, token)
			if runHasDigit {
				zhTokens = append(zhTokens, token)
			}
		}
		run = run[:0]
		runHasDigit = false
	}

	for _, r := range text {
		nextClass := classifyRune(r)
		if nextClass == scriptSeparator {
			flush()
			class = scriptSeparator
			continue
		}
		if class != scriptSeparator && class != nextClass {
			flush()
		}
		class = nextClass
		run = append(run, r)
		if unicode.IsDigit(r) {
			runHasDigit = true
		}
	}
	flush()

	return strings.Join(enTokens, " "), strings.Join(zhTokens, " ")
}

func classifyRune(r rune) scriptClass {
	switch {
	case unicode.Is(unicode.Han, r):
		return scriptHan
	case unicode.Is(unicode.Latin, r), unicode.IsDigit(r):
		return scriptLatinNumber
	default:
		return scriptSeparator
	}
}
