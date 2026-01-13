package dbtypes

import (
	"encoding/json"
	"time"
)

// OCRResultMeta 用于在 PhotoSpecificMetadata 中缓存 OCR 统计信息
type OCRResultMeta struct {
	HasOCR        bool      `json:"has_ocr"`         // 是否有 OCR 结果
	TotalCount    int       `json:"total_count"`      // 文字区域总数
	FirstText     string    `json:"first_text"`       // 第一个识别的文字（预览）
	ProcessingTime int      `json:"processing_time_ms"` // 处理耗时
	GeneratedAt   time.Time `json:"generated_at"`     // 生成时间
	ModelID       string    `json:"model_id"`         // 模型标识
}

// BoundingBoxPoint 表示多边形的一个顶点
type BoundingBoxPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// BoundingBox 表示文字区域的多边形边界框
type BoundingBox struct {
	Points []BoundingBoxPoint `json:"points"` // 通常4个点: TL, TR, BR, BL
}

// NewBoundingBox 从 OCRItem 的 [][]int 创建 BoundingBox
func NewBoundingBox(points [][]int) *BoundingBox {
	bb := &BoundingBox{}
	for _, point := range points {
		if len(point) >= 2 {
			bb.Points = append(bb.Points, BoundingBoxPoint{
				X: point[0],
				Y: point[1],
			})
		}
	}
	return bb
}

// ToIntArray 转换为 OCRItem 需要的 [][]int 格式
func (bb *BoundingBox) ToIntArray() [][]int {
	result := make([][]int, len(bb.Points))
	for i, point := range bb.Points {
		result[i] = []int{point.X, point.Y}
	}
	return result
}

// CalculateArea 计算多边形面积（使用鞋带公式）
func (bb *BoundingBox) CalculateArea() float64 {
	if len(bb.Points) < 3 {
		return 0
	}

	area := 0.0
	n := len(bb.Points)

	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += float64(bb.Points[i].X * bb.Points[j].Y)
		area -= float64(bb.Points[j].X * bb.Points[i].Y)
	}

	return area / 2.0
}

// GetCenter 获取多边形中心点
func (bb *BoundingBox) GetCenter() (int, int) {
	if len(bb.Points) == 0 {
		return 0, 0
	}

	sumX, sumY := 0, 0
	for _, point := range bb.Points {
		sumX += point.X
		sumY += point.Y
	}

	return sumX / len(bb.Points), sumY / len(bb.Points)
}

// GetBounds 获取边界矩形的坐标
func (bb *BoundingBox) GetBounds() (minX, minY, maxX, maxY int) {
	if len(bb.Points) == 0 {
		return 0, 0, 0, 0
	}

	minX, minY = bb.Points[0].X, bb.Points[0].Y
	maxX, maxY = bb.Points[0].X, bb.Points[0].Y

	for _, point := range bb.Points[1:] {
		if point.X < minX {
			minX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y > maxY {
			maxY = point.Y
		}
	}

	return minX, minY, maxX, maxY
}

// SerializeToJSON 序列化为 JSON 字符串
func (bb *BoundingBox) SerializeToJSON() ([]byte, error) {
	return json.Marshal(bb.Points)
}

// DeserializeFromJSON 从 JSON 字符串反序列化
func (bb *BoundingBox) DeserializeFromJSON(data []byte) error {
	return json.Unmarshal(data, &bb.Points)
}

// OCRTextItemMeta 表示单个 OCR 文字项的元数据
type OCRTextItemMeta struct {
	Text       string       `json:"text"`
	Confidence float32      `json:"confidence"`
	BoundingBox *BoundingBox `json:"bounding_box"`
	TextLength int          `json:"text_length"`
	Area       float64      `json:"area"`
	Position   *Position    `json:"position,omitempty"` // 额外的位置信息
}

// Position 表示文字在图片中的位置信息
type Position struct {
	Page      int     `json:"page"`       // 页码（适用于多页文档）
	Paragraph int     `json:"paragraph"`   // 段落编号
	Line      int     `json:"line"`        // 行号
	Word      int     `json:"word"`        // 词号
	ReadingOrder int  `json:"reading_order"` // 阅读顺序
}

// OCRStats 表示 OCR 处理的统计信息
type OCRStats struct {
	ModelID          string  `json:"model_id"`
	TotalAssets      int     `json:"total_assets"`
	TotalTextItems   int     `json:"total_text_items"`
	AvgItemsPerAsset float64 `json:"avg_items_per_asset"`
	MinProcessingTime int   `json:"min_processing_time_ms"`
	MaxProcessingTime int   `json:"max_processing_time_ms"`
	AvgProcessingTime float64 `json:"avg_processing_time_ms"`
}