// Package workqos defines the durable service class of catalog-owned work.
// It is independent of both River delivery and the execution governor.
package workqos

import "fmt"

// Class is ordered by urgency: a lower priority value means work should be
// considered before a higher value. The values intentionally fit River's
// supported priority range, but callers cross that boundary through Priority.
type Class uint8

const (
	Interactive Class = 1
	Background  Class = 2
	Maintenance Class = 3
)

func (class Class) Valid() bool {
	return class >= Interactive && class <= Maintenance
}

func (class Class) String() string {
	switch class {
	case Interactive:
		return "interactive"
	case Background:
		return "background"
	case Maintenance:
		return "maintenance"
	default:
		return "unknown"
	}
}

// Priority projects a valid service class onto the delivery priority scale.
func (class Class) Priority() (int, error) {
	if !class.Valid() {
		return 0, fmt.Errorf("invalid work QoS class %d", class)
	}
	return int(class), nil
}

// FromPriority restores a service class from delivery metadata.
func FromPriority(priority int) (Class, error) {
	class := Class(priority)
	if !class.Valid() {
		return 0, fmt.Errorf("invalid work priority %d", priority)
	}
	return class, nil
}
