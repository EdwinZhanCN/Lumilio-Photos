package execution

// Governor admits work.
type Governor struct{}

// NewGovernor builds the governor.
func NewGovernor() *Governor { return &Governor{} }

// Default is the process-wide governor.
func Default() *Governor { return NewGovernor() }
