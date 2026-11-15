package dbtypes

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSON stores validated JSON as SQLite TEXT instead of a byte-slice BLOB.
type JSON json.RawMessage

// Scan implements sql.Scanner.
func (j *JSON) Scan(src any) error {
	if j == nil {
		return fmt.Errorf("dbtypes.JSON: nil receiver")
	}
	if src == nil {
		*j = nil
		return nil
	}

	var value []byte
	switch source := src.(type) {
	case string:
		value = []byte(source)
	case []byte:
		value = source
	default:
		return fmt.Errorf("dbtypes.JSON: unsupported source %T", src)
	}
	if !json.Valid(value) {
		return fmt.Errorf("dbtypes.JSON: invalid JSON")
	}
	*j = append((*j)[:0], value...)
	return nil
}

// Value implements driver.Valuer and deliberately returns string so STRICT
// SQLite TEXT columns do not receive json.RawMessage as a BLOB.
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("dbtypes.JSON: invalid JSON")
	}
	return string(j), nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("dbtypes.JSON: invalid JSON")
	}
	return append([]byte(nil), j...), nil
}

func (j *JSON) UnmarshalJSON(value []byte) error {
	if j == nil {
		return fmt.Errorf("dbtypes.JSON: nil receiver")
	}
	if !json.Valid(value) {
		return fmt.Errorf("dbtypes.JSON: invalid JSON")
	}
	*j = append((*j)[:0], value...)
	return nil
}
