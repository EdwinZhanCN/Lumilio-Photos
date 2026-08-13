package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	aggregatesearch "server/internal/search"
	"server/internal/utils/imagesource"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"
)

func visualSearchRequested(params SearchAssetsParams) bool {
	return params.SimilarToAssetID != nil || params.QueryEmbedding != nil || queryImageProvided(params)
}

func queryImageProvided(params SearchAssetsParams) bool {
	return params.QueryImagePath != "" || len(params.QueryImage) > 0
}

func (s *assetService) searchVisualSimilarBrowseItems(ctx context.Context, params SearchAssetsParams) (SearchBrowseResult, error) {
	if s.semanticRetriever == nil {
		return SearchBrowseResult{}, ErrSemanticSearchUnavailable
	}

	embedding, excludeMediaItem, err := s.resolveVisualQuery(ctx, params)
	if err != nil {
		return SearchBrowseResult{}, err
	}

	filter, err := buildAggregateSearchFilter(params.QueryAssetsParams)
	if err != nil {
		return SearchBrowseResult{}, err
	}

	topK := params.TopResultsLimit
	if topK <= 0 {
		topK = 200
	}
	candidates, err := s.semanticRetriever.Retrieve(ctx, aggregatesearch.Request{
		QueryEmbedding: &embedding,
		Filter:         filter,
		TopK:           topK,
	})
	if err != nil {
		return SearchBrowseResult{}, fmt.Errorf("%w: %v", ErrSemanticSearchUnavailable, err)
	}

	candidates = aggregatesearch.FilterByCosineFloor(candidates, aggregatesearch.ImageQueryCosineFloor)
	ids := make([]uuid.UUID, 0, len(candidates))
	bestTsByAsset := make(map[uuid.UUID]*int32, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.AssetID)
		if candidate.BestTsMs != nil {
			bestTsByAsset[candidate.AssetID] = candidate.BestTsMs
		}
	}

	refs, itemByAsset, err := s.resolveMediaRefsInOrder(ctx, ids)
	if err != nil {
		return SearchBrowseResult{}, err
	}
	if excludeMediaItem != uuid.Nil {
		filtered := refs[:0]
		for _, ref := range refs {
			if ref.MediaItemID == excludeMediaItem {
				continue
			}
			filtered = append(filtered, ref)
		}
		refs = filtered
	}

	bestTsByItem := make(map[uuid.UUID]*int32, len(refs))
	for _, assetID := range ids {
		itemID, ok := itemByAsset[assetID]
		if !ok {
			continue
		}
		if _, exists := bestTsByItem[itemID]; exists {
			continue
		}
		if ts, found := bestTsByAsset[assetID]; found {
			bestTsByItem[itemID] = ts
		}
	}

	total := int64(len(refs))
	page := pageMediaRefs(refs, params.Limit, params.Offset)
	items, err := s.browseItemsForMediaRefs(ctx, page, bestTsByItem, params.IsDeleted)
	if err != nil {
		return SearchBrowseResult{}, err
	}

	return SearchBrowseResult{
		TopResults: []BrowseItem{},
		TopResultsMeta: SearchTopResultsMeta{
			Enabled:     true,
			SourceTypes: []string{aggregatesearch.SourceEmbedding},
		},
		Results:                items,
		ResultsTotalVisible:    total,
		ResultsTotalMediaItems: total,
	}, nil
}

func pageMediaRefs(refs []mediaRef, limit, offset int) []mediaRef {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(refs) {
		return []mediaRef{}
	}
	end := offset + limit
	if end > len(refs) {
		end = len(refs)
	}
	return refs[offset:end]
}

func (s *assetService) resolveVisualQuery(ctx context.Context, params SearchAssetsParams) (aggregatesearch.QueryEmbedding, uuid.UUID, error) {
	if params.QueryEmbedding != nil {
		if len(params.QueryEmbedding.Vector) == 0 {
			return aggregatesearch.QueryEmbedding{}, uuid.Nil, fmt.Errorf("query embedding is empty")
		}
		return *params.QueryEmbedding, uuid.Nil, nil
	}

	if queryImageProvided(params) {
		embedding, err := s.embedQueryImage(ctx, params)
		return embedding, uuid.Nil, err
	}

	if params.SimilarToAssetID == nil {
		return aggregatesearch.QueryEmbedding{}, uuid.Nil, fmt.Errorf("visual search query is empty")
	}

	if s.embeddingService == nil {
		return aggregatesearch.QueryEmbedding{}, uuid.Nil, fmt.Errorf("%w: embedding service not available", ErrSemanticSearchUnavailable)
	}

	primary, err := s.embeddingService.GetSearchQueryEmbedding(ctx, *params.SimilarToAssetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return aggregatesearch.QueryEmbedding{}, uuid.Nil, ErrEmbeddingMissing
		}
		return aggregatesearch.QueryEmbedding{}, uuid.Nil, err
	}
	if len(primary.Vector) == 0 {
		return aggregatesearch.QueryEmbedding{}, uuid.Nil, ErrEmbeddingMissing
	}

	excludeMediaItem := uuid.Nil
	if s.queries != nil {
		rows, mediaErr := s.queries.GetMediaItemsByAssetIDs(ctx, []uuid.UUID{*params.SimilarToAssetID})
		if mediaErr != nil {
			return aggregatesearch.QueryEmbedding{}, uuid.Nil, fmt.Errorf("resolve query media item: %w", mediaErr)
		}
		if len(rows) > 0 {
			excludeMediaItem = rows[0].MediaItemID
		}
	}

	return aggregatesearch.QueryEmbedding{
		Model:  primary.Model,
		Vector: primary.Vector,
	}, excludeMediaItem, nil
}

func (s *assetService) embedQueryImage(ctx context.Context, params SearchAssetsParams) (aggregatesearch.QueryEmbedding, error) {
	if s.lumen == nil || !s.lumen.IsTaskAvailable(types.TaskSemanticImageEmbed) {
		return aggregatesearch.QueryEmbedding{}, ErrSemanticSearchUnavailable
	}

	thumbnail, err := prepareQueryImageThumbnail(ctx, params)
	if err != nil {
		return aggregatesearch.QueryEmbedding{}, fmt.Errorf("%w: %v", ErrInvalidImageQuery, err)
	}

	mlImage, err := imagesource.ProcessMLImageTensorBytes(thumbnail, imagesource.PurposeSemantic)
	if err != nil {
		return aggregatesearch.QueryEmbedding{}, fmt.Errorf("%w: %v", ErrInvalidImageQuery, err)
	}

	result, err := s.lumen.SemanticImageEmbed(ctx, mlImage)
	if err != nil {
		return aggregatesearch.QueryEmbedding{}, fmt.Errorf("%w: %v", ErrSemanticSearchUnavailable, err)
	}
	if result == nil || len(result.Vector) == 0 {
		return aggregatesearch.QueryEmbedding{}, fmt.Errorf("%w: semantic_image_embed returned empty embedding", ErrSemanticSearchUnavailable)
	}

	return aggregatesearch.QueryEmbedding{
		Model:  result.ModelID,
		Vector: canonicalizeSemanticVector(result.Vector),
	}, nil
}

func prepareQueryImageThumbnail(ctx context.Context, params SearchAssetsParams) ([]byte, error) {
	if params.QueryImagePath != "" {
		return imagesource.PrepareSemanticThumbnail(ctx, params.QueryImagePath, params.QueryImageFilename)
	}
	if len(params.QueryImage) == 0 {
		return nil, fmt.Errorf("query image is empty")
	}
	return imagesource.PrepareSemanticThumbnailBytes(ctx, params.QueryImage, params.QueryImageFilename)
}
