package app

import "fx/execution"

// Literal builds a demand by hand.
func Literal() execution.Resources { return execution.Resources{CPU: 2} }

// Pointer allocates one.
func Pointer() *execution.Resources { return new(execution.Resources) }
