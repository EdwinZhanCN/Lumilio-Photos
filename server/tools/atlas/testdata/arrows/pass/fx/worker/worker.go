package worker

import "fx/jobs"

// Worker runs whatever its owner wires into Execute.
type Worker struct {
	Execute func(jobs.Args) error
}

// Work runs one payload.
func (w *Worker) Work(args jobs.Args) error { return w.Execute(args) }

// Guard wraps an execute function.
func Guard(execute func(jobs.Args) error) func(jobs.Args) error {
	return func(args jobs.Args) error { return execute(args) }
}

// Saver is the narrow port the service writes through.
type Saver interface{ Save(id string) error }

// Service handles one id.
type Service struct {
	saver  Saver
	notify func(string)
}

// NewService builds a Service over a Saver.
func NewService(saver Saver) *Service { return &Service{saver: saver} }

// SetNotify installs the notification hook.
func (s *Service) SetNotify(notify func(string)) { s.notify = notify }

// Handle saves and notifies.
func (s *Service) Handle(id string) error {
	s.notify(id)
	return s.saver.Save(id)
}
