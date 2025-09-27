package dbtypes

import (
	"encoding/json"
	"errors"
	"time"
)

// 资产类型枚举（与数据库中的 type 文本值对应）
type AssetType string

const (
	AssetTypePhoto AssetType = "PHOTO"
	AssetTypeVideo AssetType = "VIDEO"
	AssetTypeAudio AssetType = "AUDIO"
)

func (at AssetType) String() *string {
	str := string(at)
	return &str
}

func (at AssetType) Valid() bool {
	switch at {
	case AssetTypePhoto, AssetTypeVideo, AssetTypeAudio:
		return true
	}
	return false
}

// ✅ 方案A：大类型（零开销别名），底层就是 []byte
// 注意：别名不能挂方法，因此解码用顶层函数提供
type SpecificMetadata = json.RawMessage

// ----- 各类具体的 metadata 结构（仅用于 JSON 编解码；与持久化解耦）-----

type PhotoSpecificMetadata struct {
	TakenTime         *time.Time              `json:"taken_time,omitempty"`
	CameraModel       string                  `json:"camera_model,omitempty"`
	LensModel         string                  `json:"lens_model,omitempty"`
	ExposureTime      string                  `json:"exposure_time,omitempty"`
	FNumber           float32                 `json:"f_number,omitempty"`
	FocalLength       float32                 `json:"focal_length,omitempty"`
	IsoSpeed          int                     `json:"iso_speed,omitempty"`
	Exposure          float32                 `json:"exposure"`
	Dimensions        string                  `json:"dimensions,omitempty"`
	Resolution        string                  `json:"resolution,omitempty"`
	GPSLatitude       float64                 `json:"gps_latitude,omitempty"`
	GPSLongitude      float64                 `json:"gps_longitude,omitempty"`
	Description       string                  `json:"description,omitempty"`
	SpeciesPrediction []SpeciesPredictionMeta `json:"species_prediction,omitempty"`
	IsRAW             bool                    `json:"is_raw,omitempty"`
	Rating            int                     `json:"rating,omitempty"`
	Like              bool                    `json:"liked,omitempty"`
}

type SpeciesPredictionMeta struct {
	Label string  `json:"label"`
	Score float32 `json:"score"`
}

type VideoSpecificMetadata struct {
	Codec        string     `json:"codec,omitempty"`
	Bitrate      int        `json:"bitrate,omitempty"`
	FrameRate    float64    `json:"frame_rate,omitempty"`
	RecordedTime *time.Time `json:"recorded_time,omitempty"`
	CameraModel  string     `json:"camera_model,omitempty"`
	GPSLatitude  float64    `json:"gps_latitude,omitempty"`
	GPSLongitude float64    `json:"gps_longitude,omitempty"`
	Description  string     `json:"description,omitempty"`
}

type AudioSpecificMetadata struct {
	Codec       string `json:"codec,omitempty"`
	Bitrate     int    `json:"bitrate,omitempty"`
	SampleRate  int    `json:"sample_rate,omitempty"`
	Channels    int    `json:"channels,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	Title       string `json:"title,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Year        int    `json:"year,omitempty"`
	Description string `json:"description,omitempty"`
}

// ----- 便捷（反）序列化函数 -----
// 建议优先使用 MarshalMeta 返回 SpecificMetadata（避免到处手动转换）

// MarshalMeta 把任意 metadata struct 编为 SpecificMetadata（json.RawMessage）
func MarshalMeta(v any) (SpecificMetadata, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	return SpecificMetadata(b), err
}

// 如果你已有 []byte -> 可以直接转：SpecificMetadata(b)

// 解码为具体类型（按需使用）
func UnmarshalPhoto(b []byte) (PhotoSpecificMetadata, error) {
	var m PhotoSpecificMetadata
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func UnmarshalVideo(b []byte) (VideoSpecificMetadata, error) {
	var m VideoSpecificMetadata
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func UnmarshalAudio(b []byte) (AudioSpecificMetadata, error) {
	var m AudioSpecificMetadata
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

// 按资产类型分发解码（返回 any，调用方断言或使用类型开关）
func UnmarshalByType(t AssetType, b []byte) (any, error) {
	switch t {
	case AssetTypePhoto:
		return UnmarshalPhoto(b)
	case AssetTypeVideo:
		return UnmarshalVideo(b)
	case AssetTypeAudio:
		return UnmarshalAudio(b)
	default:
		return nil, errors.New("unknown asset type")
	}
}
