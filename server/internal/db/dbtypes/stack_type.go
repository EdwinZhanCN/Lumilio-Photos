package dbtypes

import "fmt"

// StackKind describes why assets were grouped into a stack.
type StackKind string

const (
	// StackKindBurst groups frames captured in the same camera burst.
	StackKindBurst StackKind = "burst"
	// StackKindManual groups assets explicitly placed together by the user.
	StackKindManual StackKind = "manual"
)

func (sk StackKind) String() *string {
	str := string(sk)
	return &str
}

func (sk StackKind) Valid() bool {
	switch sk {
	case StackKindBurst, StackKindManual:
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
