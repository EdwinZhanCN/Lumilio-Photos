package app

import (
	"fx/audit"
	"fx/jobs"
	"fx/store"
	"fx/worker"
)

type runtime struct{ service *worker.Service }

func (r *runtime) run(args jobs.Args) error { return r.service.Handle(args.ID) }

// Wire connects the fixture's parts.
func Wire() *worker.Worker {
	service := worker.NewService(store.NewStore())
	service.SetNotify(audit.Record)
	r := &runtime{service: service}
	return &worker.Worker{Execute: worker.Guard(r.run)}
}

// Unrelated references neither end of anything.
func Unrelated() int { return 1 }
