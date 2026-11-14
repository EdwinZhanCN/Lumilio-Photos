package execution

// Resources is a resource demand.
type Resources struct{ CPU int }

// Demand returns a named demand.
func Demand() Resources { return Resources{CPU: 1} }
