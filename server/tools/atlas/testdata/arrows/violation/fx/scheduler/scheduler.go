package scheduler

import "fx/jobs"

// Scheduler derives work.
type Scheduler struct{}

// Enqueue produces the next payload.
func (s *Scheduler) Enqueue() jobs.Args { return jobs.Args{ID: "a"} }
