package queue

import "fx/river"

// Worker runs one job.
type Worker struct{ client *river.Client[int] }

// Work enqueues child work, which it must not.
func (w *Worker) Work() error { return w.client.InsertMany([]string{"child"}) }
