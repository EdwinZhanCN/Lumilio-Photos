package river

// Workers is a worker registry.
type Workers struct{}

// AddWorker registers a worker for job args T.
func AddWorker[T any](w *Workers) {}
