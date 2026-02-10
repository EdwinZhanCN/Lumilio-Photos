package dbtypes

import (
	"time"
)

// CaptionMeta 用于在 PhotoSpecificMetadata 中缓存 AI 描述信息
type CaptionMeta struct {
	HasDescription  bool      `json:"has_description"`    // 是否有 AI 描述
	Summary         string    `json:"summary"`            // 描述摘要（前100字符）
	ModelID         string    `json:"model_id"`           // AI 模型标识
	TokensGenerated int       `json:"tokens_generated"`   // 生成的 token 数量
	Confidence      float64   `json:"confidence"`         // 置信度
	GeneratedAt     time.Time `json:"generated_at"`       // 生成时间
	ProcessingTime  int       `json:"processing_time_ms"` // 处理耗时
	FinishReason    string    `json:"finish_reason"`      // 完成原因
}

// CaptionStats 表示 AI 描述处理的统计信息
type CaptionStats struct {
	ModelID           string  `json:"model_id"`
	TotalDescriptions int     `json:"total_descriptions"`
	AvgTokens         float64 `json:"avg_tokens"`
	MinTokens         int64   `json:"min_tokens"`
	MaxTokens         int64   `json:"max_tokens"`
	AvgProcessingTime float64 `json:"avg_processing_time_ms"`
	MinProcessingTime int64   `json:"min_processing_time_ms"`
	MaxProcessingTime int64   `json:"max_processing_time_ms"`
	AvgConfidence     float64 `json:"avg_confidence"`
}

// CaptionRequest 表示 AI 描述生成请求
type CaptionRequest struct {
	AssetID     string  `json:"asset_id"`
	ImageData   []byte  `json:"image_data"`
	Prompt      string  `json:"prompt,omitempty"`
	Model       string  `json:"model,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// CaptionResponse 表示 AI 描述生成响应
type CaptionResponse struct {
	AssetID         string    `json:"asset_id"`
	Description     string    `json:"description"`
	Summary         string    `json:"summary"`
	ModelID         string    `json:"model_id"`
	Confidence      float64   `json:"confidence"`
	TokensGenerated int       `json:"tokens_generated"`
	ProcessingTime  int       `json:"processing_time_ms"`
	FinishReason    string    `json:"finish_reason"`
	PromptUsed      string    `json:"prompt_used"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// CaptionFilter 用于描述搜索的过滤器
type CaptionFilter struct {
	SearchText    string     `json:"search_text,omitempty"`
	ModelID       string     `json:"model_id,omitempty"`
	MinConfidence float64    `json:"min_confidence,omitempty"`
	MaxTokens     int        `json:"max_tokens,omitempty"`
	MinTokens     int        `json:"min_tokens,omitempty"`
	FinishReason  string     `json:"finish_reason,omitempty"`
	DateFrom      *time.Time `json:"date_from,omitempty"`
	DateTo        *time.Time `json:"date_to,omitempty"`
}

// Validate 验证过滤器参数
func (f *CaptionFilter) Validate() error {
	if f.MinConfidence < 0 || f.MinConfidence > 1 {
		return ErrInvalidConfidenceRange
	}
	if f.MaxTokens < 0 {
		return ErrInvalidMaxTokens
	}
	if f.MinTokens < 0 {
		return ErrInvalidMinTokens
	}
	return nil
}

// GenerateSummary 从完整描述生成摘要
func GenerateSummary(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}

	// 简单的摘要生成：截取前 maxLength 字符并添加省略号
	summary := description[:maxLength]

	// 尝试在单词边界截断
	for i := len(summary) - 1; i > 0; i-- {
		if summary[i] == ' ' || summary[i] == '.' || summary[i] == '!' {
			summary = summary[:i] + "..."
			break
		}
	}

	return summary
}

// EstimateTokenCount 估算文本的 token 数量
func EstimateTokenCount(text string) int {
	// 粗略估算：英文平均 1 token ≈ 4 个字符，中文平均 1 token ≈ 1.5 个字符
	if len(text) == 0 {
		return 0
	}

	// 简化估算：假设平均每个 token 4 个字符
	return (len(text) + 3) / 4
}

// 错误定义
var (
	ErrInvalidConfidenceRange = &ValidationError{"confidence must be between 0.0 and 1.0"}
	ErrInvalidMaxTokens       = &ValidationError{"max_tokens must be non-negative"}
	ErrInvalidMinTokens       = &ValidationError{"min_tokens must be non-negative"}
)

// ValidationError 验证错误
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
