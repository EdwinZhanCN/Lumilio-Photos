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

type SpecificMetadata []byte

// PhotoSpecificMetadata ----- 各类具体的 metadata 结构（仅用于 JSON 编解码；与持久化解耦）-----
type PhotoSpecificMetadata struct {
	TakenTime    *time.Time `json:"taken_time,omitempty"`
	CameraModel  string     `json:"camera_model,omitempty"`
	LensModel    string     `json:"lens_model,omitempty"`
	ExposureTime string     `json:"exposure_time,omitempty"`
	FNumber      float32    `json:"f_number,omitempty"`
	FocalLength  float32    `json:"focal_length,omitempty"`
	IsoSpeed     int        `json:"iso_speed,omitempty"`
	Exposure     float32    `json:"exposure"`
	Dimensions   string     `json:"dimensions,omitempty"`
	Resolution   string     `json:"resolution,omitempty"`
	GPSLatitude  float64    `json:"gps_latitude,omitempty"`
	GPSLongitude float64    `json:"gps_longitude,omitempty"`
	Description  string     `json:"description,omitempty"`
	IsRAW        bool       `json:"is_raw,omitempty"`
}

type VideoSpecificMetadata struct {
	Codec        string     `json:"codec,omitempty" example:"H.264"`
	Bitrate      int        `json:"bitrate,omitempty" example:"1000000"`
	FrameRate    float64    `json:"frame_rate,omitempty" example:"30.0"`
	RecordedTime *time.Time `json:"recorded_time,omitempty" example:"2023-01-01T00:00:00Z"`
	CameraModel  string     `json:"camera_model,omitempty" example:"Canon EOS 5D Mark IV"`
	GPSLatitude  float64    `json:"gps_latitude,omitempty" example:"37.7749"`
	GPSLongitude float64    `json:"gps_longitude,omitempty" example:"-122.4194"`
	Description  string     `json:"description,omitempty" example:"A beautiful sunset over the ocean"`
}

type AudioSpecificMetadata struct {
	Codec       string `json:"codec,omitempty" example:"AAC"`
	Bitrate     int    `json:"bitrate,omitempty" example:"128000"`
	SampleRate  int    `json:"sample_rate,omitempty" example:"44100"`
	Channels    int    `json:"channels,omitempty" example:"2"`
	Artist      string `json:"artist,omitempty" example:"John Doe"`
	Album       string `json:"album,omitempty" example:"Album Title"`
	Title       string `json:"title,omitempty" example:"Song Title"`
	Genre       string `json:"genre,omitempty" example:"Pop"`
	Year        int    `json:"year,omitempty" example:"2023"`
	Description string `json:"description,omitempty" example:"Song Description"`
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

// UnmarshalTo 解码到指定类型
func (s SpecificMetadata) UnmarshalTo(v any) error {
	if len(s) == 0 {
		return nil
	}
	return json.Unmarshal(s, v)
}

// UnmarshalPhoto 按照 Photo 类型解码
func (s SpecificMetadata) UnmarshalPhoto() (PhotoSpecificMetadata, error) {
	var m PhotoSpecificMetadata
	err := s.UnmarshalTo(&m)
	return m, err
}

// UnmarshalVideo 按照 Video 类型解码
func (s SpecificMetadata) UnmarshalVideo() (VideoSpecificMetadata, error) {
	var m VideoSpecificMetadata
	err := s.UnmarshalTo(&m)
	return m, err
}

// UnmarshalAudio 按照 Audio 类型解码
func (s SpecificMetadata) UnmarshalAudio() (AudioSpecificMetadata, error) {
	var m AudioSpecificMetadata
	err := s.UnmarshalTo(&m)
	return m, err
}

// UnmarshalByType 按资产类型分发解码（返回 any，调用方断言或使用类型开关）
func (s SpecificMetadata) UnmarshalByType(t AssetType) (any, error) {
	switch t {
	case AssetTypePhoto:
		return s.UnmarshalPhoto()
	case AssetTypeVideo:
		return s.UnmarshalVideo()
	case AssetTypeAudio:
		return s.UnmarshalAudio()
	default:
		return nil, errors.New("unknown asset type")
	}
}
