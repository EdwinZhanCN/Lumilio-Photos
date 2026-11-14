package jobs

// Args is the payload a scheduler hands a worker.
type Args struct{ ID string }

// Other is a payload nobody consumes.
type Other struct{}
