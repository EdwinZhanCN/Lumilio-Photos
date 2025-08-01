package queue

import (
	"context"
	"strconv"

	"github.com/riverqueue/river"
)

type PayloadArgs[T any] struct {
	Data T
	kind string
}

func (a PayloadArgs[T]) Kind() string {
	return a.kind
}

type jobWrapper[T any] struct {
	rjob *river.Job[PayloadArgs[T]]
}

func (j *jobWrapper[T]) ID() string {
	return strconv.FormatInt(j.rjob.ID, 10)
}

func (j *jobWrapper[T]) Type() JobType {
	return JobType(j.rjob.Args.kind)
}

func (j *jobWrapper[T]) Payload() T {
	return j.rjob.Args.Data
}
func (j *jobWrapper[T]) Attempt() int {
	return j.rjob.Attempt
}

type genericWorker[T any] struct {
	river.WorkerDefaults[PayloadArgs[T]]
	handler func(ctx context.Context, job Job[T]) error
}

func (w *genericWorker[T]) Work(ctx context.Context, job *river.Job[PayloadArgs[T]]) error {
	return w.handler(ctx, &jobWrapper[T]{rjob: job})
}
