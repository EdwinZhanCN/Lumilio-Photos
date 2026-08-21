package tools

import (
	"strings"
	"testing"
	"unicode/utf8"

	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateReadOCRRefBoundsCardinality(t *testing.T) {
	empty := &ref.Ref{ID: "r1_empty"}
	require.Equal(t, ref.CodeEmptySet, validateReadOCRRef(empty).Code)

	tooLarge := &ref.Ref{ID: "r2_large", AssetIDs: []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}}
	require.Equal(t, ref.CodeInvalidArgument, validateReadOCRRef(tooLarge).Code)

	bounded := &ref.Ref{ID: "r3_small", AssetIDs: []uuid.UUID{uuid.New(), uuid.New()}}
	require.Nil(t, validateReadOCRRef(bounded))
}

func TestFormatOCRDocumentsRestoresRefOrderAndReportsStatuses(t *testing.T) {
	photoID := uuid.New()
	videoID := uuid.New()
	rows := []repo.AgentReadOCRDocumentsRow{
		{
			AssetID:          photoID,
			OriginalFilename: "receipt\n\u200b.jpg",
			Type:             "PHOTO",
			HasOcrResult:     1,
			RegionCount:      2,
			TextContent:      "first\tprovider\nline",
		},
		{
			AssetID:          photoID,
			OriginalFilename: "receipt.jpg",
			Type:             "PHOTO",
			HasOcrResult:     1,
			RegionCount:      2,
			TextContent:      "second provider line",
		},
		{
			AssetID:          videoID,
			OriginalFilename: "clip.mp4",
			Type:             "VIDEO",
			HasOcrResult:     0,
		},
	}

	documents := formatOCRDocuments([]uuid.UUID{videoID, photoID}, rows)

	require.Equal(t, []ReadOCRDocument{
		{
			Position: 1,
			Filename: "clip.mp4",
			Status:   ocrStatusUnsupportedType,
			Lines:    []string{},
		},
		{
			Position:    2,
			Filename:    "receipt .jpg",
			Status:      ocrStatusAvailable,
			RegionCount: 2,
			Lines:       []string{"first provider line", "second provider line"},
		},
	}, documents)
}

func TestFormatOCRDocumentsDistinguishesMissingAndZeroRegionResults(t *testing.T) {
	missingID := uuid.New()
	emptyID := uuid.New()
	rows := []repo.AgentReadOCRDocumentsRow{{
		AssetID:          missingID,
		OriginalFilename: "missing.jpg",
		Type:             "PHOTO",
		HasOcrResult:     0,
	}, {
		AssetID:          emptyID,
		OriginalFilename: "blank.jpg",
		Type:             "PHOTO",
		HasOcrResult:     1,
	}}

	documents := formatOCRDocuments([]uuid.UUID{missingID, emptyID}, rows)

	require.Equal(t, ocrStatusNotAvailable, documents[0].Status)
	require.Equal(t, ocrStatusAvailable, documents[1].Status)
	require.Empty(t, documents[0].Lines)
	require.Empty(t, documents[1].Lines)
}

func TestFormatOCRDocumentsEnforcesLineAndGlobalRuneBudgets(t *testing.T) {
	firstID := uuid.New()
	secondID := uuid.New()
	rows := make([]repo.AgentReadOCRDocumentsRow, 0, 14)
	for range 13 {
		rows = append(rows, repo.AgentReadOCRDocumentsRow{
			AssetID:          firstID,
			OriginalFilename: "large.jpg",
			Type:             "PHOTO",
			HasOcrResult:     1,
			RegionCount:      13,
			TextContent:      strings.Repeat("界", maxOCRLineRunes+50),
		})
	}
	rows = append(rows, repo.AgentReadOCRDocumentsRow{
		AssetID:          secondID,
		OriginalFilename: "later.jpg",
		Type:             "PHOTO",
		HasOcrResult:     1,
		RegionCount:      1,
		TextContent:      "later line",
	})

	documents := formatOCRDocuments([]uuid.UUID{firstID, secondID}, rows)

	totalRunes := 0
	for _, document := range documents {
		for _, line := range document.Lines {
			require.LessOrEqual(t, utf8.RuneCountInString(line), maxOCRLineRunes)
			totalRunes += utf8.RuneCountInString(line)
		}
	}
	require.LessOrEqual(t, totalRunes, maxOCRTotalRunes)
	require.Len(t, documents[0].Lines, 12)
	require.True(t, documents[0].Truncated)
	require.Empty(t, documents[1].Lines)
	require.True(t, documents[1].Truncated)
}
