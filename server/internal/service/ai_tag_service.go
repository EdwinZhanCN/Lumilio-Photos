package service

import (
	"context"
	"fmt"
	"strings"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
)

type AIGeneratedTag struct {
	Name       string
	Confidence float32
	Source     string
	Category   string
}

type AIGeneratedTagService interface {
	ReplaceAssetAIGeneratedTags(ctx context.Context, assetID pgtype.UUID, tags []AIGeneratedTag, sources []string) error
}

type aiGeneratedTagService struct {
	queries *repo.Queries
}

func NewAIGeneratedTagService(queries *repo.Queries) AIGeneratedTagService {
	return &aiGeneratedTagService{queries: queries}
}

func (s *aiGeneratedTagService) ReplaceAssetAIGeneratedTags(ctx context.Context, assetID pgtype.UUID, tags []AIGeneratedTag, sources []string) error {
	normalizedSources := normalizeAssetTagSources(sources)
	if len(normalizedSources) > 0 {
		if err := s.queries.RemoveAssetTagsBySources(ctx, repo.RemoveAssetTagsBySourcesParams{
			AssetID: assetID,
			Sources: normalizedSources,
		}); err != nil {
			return fmt.Errorf("remove existing ai tags: %w", err)
		}
	}

	deduped := dedupeAIGeneratedTags(tags)
	for _, tag := range deduped {
		dbTag, err := s.getOrCreateTagByName(ctx, tag.Name, tag.Category)
		if err != nil {
			return err
		}

		confidenceNumeric := pgtype.Numeric{}
		if err := confidenceNumeric.Scan(fmt.Sprintf("%.3f", tag.Confidence)); err != nil {
			return fmt.Errorf("convert confidence for tag %q: %w", tag.Name, err)
		}

		if err := s.queries.AddTagToAsset(ctx, repo.AddTagToAssetParams{
			AssetID:    assetID,
			TagID:      dbTag.TagID,
			Confidence: confidenceNumeric,
			Source:     normalizeAssetTagSource(tag.Source),
		}); err != nil {
			return fmt.Errorf("attach tag %q to asset: %w", tag.Name, err)
		}
	}

	return nil
}

func (s *aiGeneratedTagService) getOrCreateTagByName(ctx context.Context, name, category string) (*repo.Tag, error) {
	tag, err := s.queries.GetTagByName(ctx, name)
	if err == nil {
		return &tag, nil
	}

	isAIGenerated := true
	params := repo.CreateTagParams{
		TagName:       name,
		IsAiGenerated: &isAIGenerated,
	}
	if category != "" {
		params.Category = &category
	}

	dbTag, err := s.queries.CreateTag(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create tag %q: %w", name, err)
	}

	return &dbTag, nil
}

func dedupeAIGeneratedTags(tags []AIGeneratedTag) []AIGeneratedTag {
	deduped := make([]AIGeneratedTag, 0, len(tags))
	indexByName := make(map[string]int, len(tags))

	for _, tag := range tags {
		tag.Name = strings.TrimSpace(tag.Name)
		tag.Source = strings.TrimSpace(tag.Source)
		tag.Category = strings.TrimSpace(tag.Category)
		if tag.Name == "" || tag.Source == "" {
			continue
		}

		key := strings.ToLower(tag.Name)
		if idx, ok := indexByName[key]; ok {
			if tag.Confidence > deduped[idx].Confidence {
				deduped[idx] = tag
			}
			continue
		}

		indexByName[key] = len(deduped)
		deduped = append(deduped, tag)
	}

	return deduped
}

func normalizeAssetTagSources(sources []string) []string {
	normalized := make([]string, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		value := normalizeAssetTagSource(source)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func normalizeAssetTagSource(source string) string {
	switch strings.TrimSpace(source) {
	case "":
		return ""
	case "system", "user", "ai":
		return strings.TrimSpace(source)
	default:
		return "ai"
	}
}
