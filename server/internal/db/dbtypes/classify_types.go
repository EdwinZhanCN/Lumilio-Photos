package dbtypes

import (
	"time"
)

// ClassifyResultMeta 用于在 PhotoSpecificMetadata 中缓存分类统计信息
type ClassifyResultMeta struct {
	HasClassification bool      `json:"has_classification"` // 是否有分类结果
	TopLabel          string    `json:"top_label"`          // 最高置信度的标签
	TopScore          float32   `json:"top_score"`          // 最高置信度分数
	TotalLabels       int       `json:"total_labels"`       // 标签总数
	ProcessingTime    int       `json:"processing_time_ms"` // 处理耗时
	GeneratedAt       time.Time `json:"generated_at"`       // 生成时间
	ModelID           string    `json:"model_id"`           // 模型标识
}

// SpeciesPredictionMeta 物种识别结果
type SpeciesPredictionMeta struct {
	Label string  `json:"label"` // 物种标签
	Score float32 `json:"score"` // 置信度分数
}

// ClassificationMeta 通用图像分类结果
type ClassificationMeta struct {
	Label    string  `json:"label"`              // 分类标签
	Score    float32 `json:"score"`              // 置信度分数
	Category string  `json:"category,omitempty"` // 分类类别（可选）
}

// ClassifyStats 表示分类处理的统计信息
type ClassifyStats struct {
	ModelID           string  `json:"model_id"`
	TotalAssets       int     `json:"total_assets"`
	TotalPredictions  int     `json:"total_predictions"`
	AvgLabelsPerAsset float64 `json:"avg_labels_per_asset"`
	MinProcessingTime int     `json:"min_processing_time_ms"`
	MaxProcessingTime int     `json:"max_processing_time_ms"`
	AvgProcessingTime float64 `json:"avg_processing_time_ms"`
}
