package app

import "fx/execution"

// Run uses the process-wide governor.
func Run() *execution.Governor { return execution.Default() }
