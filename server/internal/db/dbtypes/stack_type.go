package dbtypes

import "fmt"

// StackKind describes why assets were grouped into a stack.
type StackKind string

const (
	// StackKindRawJpeg groups related camera originals such as RAW + JPEG pairs.
	StackKindRawJpeg StackKind = "raw_jpeg"
	// StackKindLivePhoto groups the still image and companion video for a Live Photo.
	StackKindLivePhoto StackKind = "live_photo"
	// StackKindManual groups assets explicitly placed together by the user.
	StackKindManual StackKind = "manual"
)

func (sk StackKind) String() *string {
	str := string(sk)
	return &str
}

func (sk StackKind) Valid() bool {
	switch sk {
	case StackKindRawJpeg, StackKindLivePhoto, StackKindManual:
		return true
	default:
		return false
	}
}

func (sk *StackKind) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*sk = StackKind(s)
	case string:
		*sk = StackKind(s)
	default:
		return fmt.Errorf("unsupported scan type for StackKind: %T", src)
	}
	return nil
}
