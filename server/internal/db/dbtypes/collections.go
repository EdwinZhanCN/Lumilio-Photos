package dbtypes

import (
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

	"github.com/google/uuid"
)

// UUIDs stores UUID collections as validated JSON arrays in SQLite TEXT.
type UUIDs []uuid.UUID

// UUIDsJSONParam encodes a non-empty UUID list for SQLite JSON1 query
// parameters. Empty lists return nil so optional collection filters are
// disabled instead of matching no rows.
func UUIDsJSONParam(values []uuid.UUID) *string {
	if len(values) == 0 {
		return nil
	}
	encoded, err := json.Marshal(UUIDs(values))
	if err != nil {
		panic(fmt.Sprintf("dbtypes.UUIDsJSONParam: %v", err))
	}
	text := string(encoded)
	return &text
}

func (u *UUIDs) Scan(src any) error {
	if u == nil {
		return fmt.Errorf("dbtypes.UUIDs: nil receiver")
	}
	value, err := jsonSource(src)
	if err != nil {
		return fmt.Errorf("dbtypes.UUIDs: %w", err)
	}
	if value == nil {
		*u = nil
		return nil
	}
	if err := json.Unmarshal(value, u); err != nil {
		return fmt.Errorf("dbtypes.UUIDs: decode: %w", err)
	}
	return nil
}

func (u UUIDs) Value() (driver.Value, error) {
	if u == nil {
		return "[]", nil
	}
	value, err := json.Marshal(u)
	if err != nil {
		return nil, fmt.Errorf("dbtypes.UUIDs: encode: %w", err)
	}
	return string(value), nil
}

// Strings stores string collections as validated JSON arrays in SQLite TEXT.
type Strings []string

// StringsJSONParam encodes a non-empty string list for SQLite JSON1 query
// parameters. Empty lists return nil so optional collection filters are
// disabled instead of matching no rows.
func StringsJSONParam(values []string) *string {
	if len(values) == 0 {
		return nil
	}
	encoded, err := json.Marshal(Strings(values))
	if err != nil {
		panic(fmt.Sprintf("dbtypes.StringsJSONParam: %v", err))
	}
	text := string(encoded)
	return &text
}

// IntegersJSONParam encodes a non-empty integer list for SQLite JSON1 query
// parameters. Empty lists return nil so json_each(NULL) matches no rows.
func IntegersJSONParam[T ~int | ~int32 | ~int64](values []T) *string {
	if len(values) == 0 {
		return nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		panic(fmt.Sprintf("dbtypes.IntegersJSONParam: %v", err))
	}
	text := string(encoded)
	return &text
}

func (s *Strings) Scan(src any) error {
	if s == nil {
		return fmt.Errorf("dbtypes.Strings: nil receiver")
	}
	value, err := jsonSource(src)
	if err != nil {
		return fmt.Errorf("dbtypes.Strings: %w", err)
	}
	if value == nil {
		*s = nil
		return nil
	}
	if err := json.Unmarshal(value, s); err != nil {
		return fmt.Errorf("dbtypes.Strings: decode: %w", err)
	}
	return nil
}

func (s Strings) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	value, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("dbtypes.Strings: encode: %w", err)
	}
	return string(value), nil
}

// Vector stores little-endian float32 values in an authoritative SQLite BLOB.
type Vector []float32

func NewVector(values []float32) Vector {
	return append(Vector(nil), values...)
}

func (v Vector) Slice() []float32 {
	return append([]float32(nil), v...)
}

func (v *Vector) Scan(src any) error {
	if v == nil {
		return fmt.Errorf("dbtypes.Vector: nil receiver")
	}
	if src == nil {
		*v = nil
		return nil
	}
	value, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("dbtypes.Vector: unsupported source %T", src)
	}
	if len(value)%4 != 0 {
		return fmt.Errorf("dbtypes.Vector: BLOB length %d is not divisible by 4", len(value))
	}
	result := make(Vector, len(value)/4)
	for index := range result {
		bits := binary.LittleEndian.Uint32(value[index*4:])
		result[index] = math.Float32frombits(bits)
	}
	*v = result
	return nil
}

func (v Vector) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	value := make([]byte, len(v)*4)
	for index, item := range v {
		binary.LittleEndian.PutUint32(value[index*4:], math.Float32bits(item))
	}
	return value, nil
}

func jsonSource(src any) ([]byte, error) {
	switch source := src.(type) {
	case nil:
		return nil, nil
	case string:
		return []byte(source), nil
	case []byte:
		return source, nil
	default:
		return nil, fmt.Errorf("unsupported source %T", src)
	}
}
