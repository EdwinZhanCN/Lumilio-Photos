package bleveocr

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSplitTextRoutesScriptsWithoutDuplicatingWholeText(t *testing.T) {
	textEN, textZH := SplitText("北京 北 Starbucks 2025 X-T5")

	require.Contains(t, textEN, "Starbucks")
	require.Contains(t, textEN, "2025")
	require.Contains(t, textEN, "T5")
	require.NotContains(t, textEN, "北京")

	require.Contains(t, textZH, "北京")
	require.NotContains(t, textZH, " 北 ")
	require.Contains(t, textZH, "2025")
	require.Contains(t, textZH, "T5")
	require.NotContains(t, textZH, "Starbucks")
}

func TestEnglishAnalyzerStemmingCaseAndStopWords(t *testing.T) {
	index := newTestIndex(t)
	document := BuildDocument(SourceDocument{
		AssetID:   uuid.NewString(),
		OwnerID:   1,
		AssetType: "PHOTO",
		Revision:  1,
		TextItems: []string{"Running JUMPED over the cameras"},
	})
	require.NoError(t, index.Apply([]Mutation{{AssetID: document.AssetID, Document: &document}}))

	page, err := index.SearchPage(context.Background(), "RUN jumps", BasicFilters{}, QueryStrict, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{document.AssetID}, hitIDs(page.Hits))

	page, err = index.SearchPage(context.Background(), "the", BasicFilters{}, QueryStrict, 0, 10)
	require.NoError(t, err)
	require.Empty(t, page.Hits)
}

func TestMixedLanguageQueryIgnoresAnalyzerEmptyLanguage(t *testing.T) {
	index := newTestIndex(t)
	document := BuildDocument(SourceDocument{
		AssetID:   uuid.NewString(),
		OwnerID:   1,
		AssetType: "PHOTO",
		Revision:  1,
		TextItems: []string{"北京"},
	})
	require.NoError(t, index.Apply([]Mutation{{AssetID: document.AssetID, Document: &document}}))

	page, err := index.SearchPage(context.Background(), "北京 the", BasicFilters{}, QueryStrict, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{document.AssetID}, hitIDs(page.Hits))
}

func TestChineseBigramSimplifiedTraditionalAndSingleCharacterSuppression(t *testing.T) {
	index := newTestIndex(t)
	simplified := BuildDocument(SourceDocument{
		AssetID:   uuid.NewString(),
		OwnerID:   1,
		AssetType: "PHOTO",
		Revision:  1,
		TextItems: []string{"北京星巴克"},
	})
	traditional := BuildDocument(SourceDocument{
		AssetID:   uuid.NewString(),
		OwnerID:   1,
		AssetType: "PHOTO",
		Revision:  1,
		TextItems: []string{"臺北咖啡館"},
	})
	require.NoError(t, index.Apply([]Mutation{
		{AssetID: simplified.AssetID, Document: &simplified},
		{AssetID: traditional.AssetID, Document: &traditional},
	}))

	page, err := index.SearchPage(context.Background(), "北京", BasicFilters{}, QueryStrict, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{simplified.AssetID}, hitIDs(page.Hits))

	page, err = index.SearchPage(context.Background(), "臺北", BasicFilters{}, QueryStrict, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{traditional.AssetID}, hitIDs(page.Hits))

	page, err = index.SearchPage(context.Background(), "北", BasicFilters{}, QueryStrict, 0, 10)
	require.NoError(t, err)
	require.Empty(t, page.Hits)

	singleHan := BuildDocument(SourceDocument{
		AssetID:   uuid.NewString(),
		OwnerID:   1,
		AssetType: "PHOTO",
		Revision:  1,
		TextItems: []string{"北"},
	})
	require.False(t, singleHan.HasSearchableText())
}

func TestMixedLanguageNumericModelAndBasicFilters(t *testing.T) {
	index := newTestIndex(t)
	repositoryA := uuid.NewString()
	repositoryB := uuid.NewString()
	live := BuildDocument(SourceDocument{
		AssetID:      uuid.NewString(),
		OwnerID:      7,
		RepositoryID: repositoryA,
		AssetType:    "PHOTO",
		Revision:     1,
		TextItems:    []string{"北京 Starbucks X-T5 2025"},
	})
	wrongOwner := live
	wrongOwner.AssetID = uuid.NewString()
	wrongOwner.OwnerID = 8
	wrongRepository := live
	wrongRepository.AssetID = uuid.NewString()
	wrongRepository.RepositoryID = repositoryB
	deleted := live
	deleted.AssetID = uuid.NewString()
	deleted.IsDeleted = true
	require.NoError(t, index.Apply([]Mutation{
		{AssetID: live.AssetID, Document: &live},
		{AssetID: wrongOwner.AssetID, Document: &wrongOwner},
		{AssetID: wrongRepository.AssetID, Document: &wrongRepository},
		{AssetID: deleted.AssetID, Document: &deleted},
	}))

	ownerID := int32(7)
	assetType := "PHOTO"
	page, err := index.SearchPage(context.Background(), "北京 Starbucks T5 2025", BasicFilters{
		OwnerID:      &ownerID,
		RepositoryID: &repositoryA,
		AssetType:    &assetType,
	}, QueryStrict, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{live.AssetID}, hitIDs(page.Hits))

	page, err = index.SearchPage(context.Background(), "北京 Starbucks T5 2025", BasicFilters{
		OwnerID:      &ownerID,
		RepositoryID: &repositoryA,
		AssetType:    &assetType,
		IsDeleted:    true,
	}, QueryStrict, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{deleted.AssetID}, hitIDs(page.Hits))
}

func newTestIndex(t *testing.T) *Index {
	t.Helper()
	indexMapping, err := NewMapping()
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "ocr")
	raw, err := bleve.New(path, indexMapping)
	require.NoError(t, err)
	require.NoError(t, raw.SetInternal(mappingVersionKey, []byte(MappingVersion)))
	index := &Index{path: path, bleve: raw}
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	return index
}

func hitIDs(hits []Hit) []string {
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.AssetID)
	}
	return ids
}
