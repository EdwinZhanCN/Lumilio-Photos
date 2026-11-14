package queue

import "fx/river"

// Schedule is the only producer.
func Schedule(c *river.Client[int]) error { return c.InsertMany(nil) }
