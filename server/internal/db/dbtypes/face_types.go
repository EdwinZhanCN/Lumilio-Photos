package dbtypes

import (
	"encoding/json"
	"time"
)

// FaceResultMeta 用于在 PhotoSpecificMetadata 中缓存人脸识别统计信息
type FaceResultMeta struct {
	HasFaces       bool      `json:"has_faces"`         // 是否检测到人脸
	TotalFaces     int       `json:"total_faces"`        // 检测到的人脸总数
	HasPrimaryFace bool      `json:"has_primary_face"`   // 是否有主要人脸
	ProcessingTime int       `json:"processing_time_ms"`  // 处理耗时
	GeneratedAt    time.Time `json:"generated_at"`       // 生成时间
	ModelID        string    `json:"model_id"`           // 模型标识
}

// FaceBoundingBox 表示人脸边界框（兼容 lumen-sdk 格式）
type FaceBoundingBox struct {
	X1 float32 `json:"x1"` // 左上角 X 坐标
	Y1 float32 `json:"y1"` // 左上角 Y 坐标
	X2 float32 `json:"x2"` // 右下角 X 坐标
	Y2 float32 `json:"y2"` // 右下角 Y 坐标
}

// NewFaceBoundingBoxFromLumen 从 lumen-sdk 的 []float32 创建 FaceBoundingBox
func NewFaceBoundingBoxFromLumen(bbox []float32) *FaceBoundingBox {
	if len(bbox) < 4 {
		return nil
	}
	return &FaceBoundingBox{
		X1: bbox[0],
		Y1: bbox[1],
		X2: bbox[2],
		Y2: bbox[3],
	}
}

// ToLumenFormat 转换为 lumen-sdk 格式的 []float32
func (fb *FaceBoundingBox) ToLumenFormat() []float32 {
	return []float32{fb.X1, fb.Y1, fb.X2, fb.Y2}
}

// GetWidth 获取边界框宽度
func (fb *FaceBoundingBox) GetWidth() float32 {
	return fb.X2 - fb.X1
}

// GetHeight 获取边界框高度
func (fb *FaceBoundingBox) GetHeight() float32 {
	return fb.Y2 - fb.Y1
}

// GetArea 获取边界框面积
func (fb *FaceBoundingBox) GetArea() float32 {
	return fb.GetWidth() * fb.GetHeight()
}

// GetCenter 获取中心点坐标
func (fb *FaceBoundingBox) GetCenter() (float32, float32) {
	return (fb.X1 + fb.X2) / 2, (fb.Y1 + fb.Y2) / 2
}

// SerializeToJSON 序列化为 JSON 字符串
func (fb *FaceBoundingBox) SerializeToJSON() ([]byte, error) {
	return json.Marshal(map[string]float32{
		"x":      fb.X1,
		"y":      fb.Y1,
		"width":  fb.GetWidth(),
		"height": fb.GetHeight(),
	})
}

// FaceLandmarks 表示面部关键点
type FaceLandmarks struct {
	Points []float32 `json:"points"` // 关键点坐标数组 [x1,y1,x2,y2,...]
}

// NewFaceLandmarksFromLumen 从 lumen-sdk 的 []float32 创建 FaceLandmarks
func NewFaceLandmarksFromLumen(landmarks []float32) *FaceLandmarks {
	if len(landmarks) == 0 {
		return nil
	}
	return &FaceLandmarks{
		Points: landmarks,
	}
}

// GetPointCount 获取关键点数量
func (fl *FaceLandmarks) GetPointCount() int {
	return len(fl.Points) / 2
}

// GetPoint 获取指定索引的关键点坐标
func (fl *FaceLandmarks) GetPoint(index int) (float32, float32) {
	if index*2+1 >= len(fl.Points) {
		return 0, 0
	}
	return fl.Points[index*2], fl.Points[index*2+1]
}

// SerializeToJSON 序列化为 JSON 字符串
func (fl *FaceLandmarks) SerializeToJSON() ([]byte, error) {
	if fl == nil {
		return nil, nil
	}
	return json.Marshal(fl.Points)
}

// PoseAngles 表示头部姿态角度
type PoseAngles struct {
	Yaw   float32 `json:"yaw"`   // 偏航角（左右转头）
	Pitch float32 `json:"pitch"` // 俯仰角（上下点头）
	Roll  float32 `json:"roll"`  // 翻滚角（左右倾斜）
}

// SerializeToJSON 序列化为 JSON 字符串
func (pa *PoseAngles) SerializeToJSON() ([]byte, error) {
	if pa == nil {
		return nil, nil
	}
	return json.Marshal(pa)
}

// FaceItemMeta 表示单个人脸的元数据（用于内部处理）
type FaceItemMeta struct {
	ID            int32              `json:"id"`
	FaceID        string             `json:"face_id,omitempty"`
	BoundingBox   *FaceBoundingBox   `json:"bounding_box"`
	Confidence    float32            `json:"confidence"`
	Landmarks     *FaceLandmarks     `json:"landmarks,omitempty"`
	Embedding     []float32          `json:"embedding,omitempty"`
	EmbeddingModel string            `json:"embedding_model,omitempty"`
	IsPrimary     bool               `json:"is_primary"`
	AgeGroup      string             `json:"age_group,omitempty"`       // 预留字段
	Gender        string             `json:"gender,omitempty"`         // 预留字段
	Ethnicity     string             `json:"ethnicity,omitempty"`      // 预留字段
	Expression    string             `json:"expression,omitempty"`     // 预留字段
	FaceSize      int32              `json:"face_size"`
	QualityScore  float32            `json:"quality_score,omitempty"`  // 预留字段
	BlurScore     float32            `json:"blur_score,omitempty"`     // 预留字段
	PoseAngles    *PoseAngles        `json:"pose_angles,omitempty"`    // 预留字段
	CreatedAt     time.Time          `json:"created_at"`
}

// FaceStats 表示人脸检测处理的统计信息
type FaceStats struct {
	ModelID           string  `json:"model_id"`
	TotalAssets       int     `json:"total_assets"`
	TotalFaces        int     `json:"total_faces"`
	AvgFacesPerAsset  float64 `json:"avg_faces_per_asset"`
	MinProcessingTime int     `json:"min_processing_time_ms"`
	MaxProcessingTime int     `json:"max_processing_time_ms"`
	AvgProcessingTime float64 `json:"avg_processing_time_ms"`
}

// FaceDemographics 表示人脸人口统计学信息
type FaceDemographics struct {
	AgeGroup     string  `json:"age_group"`
	Gender       string  `json:"gender"`
	Ethnicity    string  `json:"ethnicity"`
	Count        int     `json:"count"`
	AvgConfidence float32 `json:"avg_confidence"`
}

// FaceClusterMeta 表示人脸聚类元数据
type FaceClusterMeta struct {
	ClusterID      int32     `json:"cluster_id"`
	ClusterName    string    `json:"cluster_name"`
	MemberCount    int       `json:"member_count"`
	RepresentativeFaceID int32 `json:"representative_face_id"`
	IsConfirmed    bool      `json:"is_confirmed"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SerializeJSON 通用的 JSON 序列化函数
func SerializeJSON(data interface{}) ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	return json.Marshal(data)
}

// DeserializeJSON 通用的 JSON 反序列化函数
func DeserializeJSON(data []byte, target interface{}) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}