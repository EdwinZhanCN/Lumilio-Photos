package dbtypes

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Timestamp stores UTC time as Unix microseconds in SQLite.
type Timestamp struct {
	time.Time
}

// NewTimestamp normalizes a time to UTC with SQLite's microsecond precision.
func NewTimestamp(value time.Time) Timestamp {
	return Timestamp{Time: value.UTC().Truncate(time.Microsecond)}
}

// Scan implements sql.Scanner.
func (t *Timestamp) Scan(src any) error {
	if t == nil {
		return fmt.Errorf("dbtypes.Timestamp: nil receiver")
	}

	var micros int64
	switch value := src.(type) {
	case int64:
		micros = value
	case []byte:
		if _, err := fmt.Sscan(string(value), &micros); err != nil {
			return fmt.Errorf("dbtypes.Timestamp: scan bytes %q: %w", value, err)
		}
	case string:
		if _, err := fmt.Sscan(value, &micros); err != nil {
			return fmt.Errorf("dbtypes.Timestamp: scan string %q: %w", value, err)
		}
	default:
		return fmt.Errorf("dbtypes.Timestamp: unsupported source %T", src)
	}

	t.Time = time.UnixMicro(micros).UTC()
	return nil
}

// Value implements driver.Valuer.
func (t Timestamp) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, fmt.Errorf("dbtypes.Timestamp: zero time")
	}
	return t.Time.UTC().Truncate(time.Microsecond).UnixMicro(), nil
}
