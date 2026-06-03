package dto

import "server/internal/service"

// ClassifierPreviewRequestDTO is an ad-hoc zero-shot classifier evaluated live
// over the library, used to tune prompts and thresholds before saving.
type ClassifierPreviewRequestDTO struct {
	PositivePrompts []string `json:"positive_prompts" binding:"required,min=1,dive,required"`
	NegativePrompts []string `json:"negative_prompts"`
	Threshold       float64  `json:"threshold"`
	Limit           int      `json:"limit"`
}

// ClassifierPreviewMatchDTO is one asset that matched the preview classifier.
type ClassifierPreviewMatchDTO struct {
	AssetID string  `json:"asset_id"`
	Score   float64 `json:"score"`
}

// ClassifierPreviewResponseDTO is the ranked result of a preview run.
type ClassifierPreviewResponseDTO struct {
	Count   int                         `json:"count"`
	Matches []ClassifierPreviewMatchDTO `json:"matches"`
}

// ToClassifierPreviewResponseDTO maps service matches to the response DTO.
func ToClassifierPreviewResponseDTO(matches []service.ClassifierPreviewMatch) ClassifierPreviewResponseDTO {
	out := ClassifierPreviewResponseDTO{
		Count:   len(matches),
		Matches: make([]ClassifierPreviewMatchDTO, 0, len(matches)),
	}
	for _, m := range matches {
		out.Matches = append(out.Matches, ClassifierPreviewMatchDTO{
			AssetID: m.AssetID.String(),
			Score:   m.Score,
		})
	}
	return out
}
