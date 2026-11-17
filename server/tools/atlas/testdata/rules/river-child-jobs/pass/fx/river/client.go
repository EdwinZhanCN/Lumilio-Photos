package river

// Client is a generic job client.
type Client[T any] struct{}

// InsertMany enqueues jobs.
func (c *Client[T]) InsertMany(jobs []string) error { return nil }
