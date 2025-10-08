package errgroup

import (
	"sync"
)

// FaultTolerantGroup is an errgroup that allows tasks to fail independently
type FaultTolerantGroup struct {
	tasks []func() error
	mu    sync.Mutex
}

// NewFaultTolerant creates a new fault-tolerant group
func NewFaultTolerant() *FaultTolerantGroup {
	return &FaultTolerantGroup{
		tasks: make([]func() error, 0),
	}
}

// Go adds a task to the group
func (g *FaultTolerantGroup) Go(fn func() error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tasks = append(g.tasks, fn)
}

// Wait executes all tasks and returns individual errors
func (g *FaultTolerantGroup) Wait() []error {
	g.mu.Lock()
	tasks := g.tasks
	g.mu.Unlock()

	var wg sync.WaitGroup
	errorChan := make(chan error, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(fn func() error) {
			defer wg.Done()
			if err := fn(); err != nil {
				errorChan <- err
			}
		}(task)
	}

	wg.Wait()
	close(errorChan)

	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	return errors
}

// WaitWithResults executes tasks and returns results with individual errors
func (g *FaultTolerantGroup) WaitWithResults() map[int]error {
	g.mu.Lock()
	tasks := g.tasks
	g.mu.Unlock()

	var wg sync.WaitGroup
	resultChan := make(chan struct {
		index int
		err   error
	}, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(index int, fn func() error) {
			defer wg.Done()
			err := fn()
			resultChan <- struct {
				index int
				err   error
			}{index: index, err: err}
		}(i, task)
	}

	wg.Wait()
	close(resultChan)

	results := make(map[int]error)
	for result := range resultChan {
		if result.err != nil {
			results[result.index] = result.err
		}
	}

	return results
}
