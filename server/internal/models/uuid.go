package models

import (
	"fmt"
	"github.com/google/uuid"
)

// UUID is an alias for uuid.UUID to potentially add custom methods
type UUID = uuid.UUID

// ParseUUID parses a string into a UUID
func ParseUUID(id string) (UUID, error) {
	uuid, err := uuid.Parse(id)
	if err != nil {
		return uuid, fmt.Errorf("invalid UUID format: %w", err)
	}
	return uuid, nil
}