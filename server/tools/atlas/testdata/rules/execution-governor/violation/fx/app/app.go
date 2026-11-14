package app

import "fx/execution"

// Run builds a second governor.
func Run() *execution.Governor { return execution.NewGovernor() }
