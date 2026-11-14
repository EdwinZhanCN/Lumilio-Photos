package app

import "fx/execution"

// Literal uses the named demand.
func Literal() execution.Resources { return execution.Demand() }
