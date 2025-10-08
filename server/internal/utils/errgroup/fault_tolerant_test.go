package errgroup_test

import (
	"errors"
	"fmt"
	"testing"

	"server/internal/utils/errgroup"

	"github.com/stretchr/testify/assert"
)

func TestFaultTolerantGroup(t *testing.T) {
	t.Run("All tasks succeed", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		results := make([]int, 0)
		g.Go(func() error {
			results = append(results, 1)
			return nil
		})
		g.Go(func() error {
			results = append(results, 2)
			return nil
		})
		g.Go(func() error {
			results = append(results, 3)
			return nil
		})

		errors := g.Wait()
		assert.Empty(t, errors, "Should have no errors when all tasks succeed")
		assert.Len(t, results, 3, "All tasks should have executed")
		assert.ElementsMatch(t, []int{1, 2, 3}, results, "All tasks should have completed")
	})

	t.Run("Some tasks fail", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		successCount := 0
		g.Go(func() error {
			successCount++
			return nil
		})
		g.Go(func() error {
			successCount++
			return errors.New("task 2 failed")
		})
		g.Go(func() error {
			successCount++
			return nil
		})
		g.Go(func() error {
			successCount++
			return errors.New("task 4 failed")
		})

		errors := g.Wait()
		assert.Len(t, errors, 2, "Should have 2 errors")
		assert.Equal(t, 4, successCount, "All tasks should have executed despite failures")

		// Check error messages
		errorMessages := make([]string, len(errors))
		for i, err := range errors {
			errorMessages[i] = err.Error()
		}
		assert.ElementsMatch(t, []string{"task 2 failed", "task 4 failed"}, errorMessages)
	})

	t.Run("All tasks fail", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		executionCount := 0
		g.Go(func() error {
			executionCount++
			return errors.New("error 1")
		})
		g.Go(func() error {
			executionCount++
			return errors.New("error 2")
		})
		g.Go(func() error {
			executionCount++
			return errors.New("error 3")
		})

		errors := g.Wait()
		assert.Len(t, errors, 3, "Should have 3 errors")
		assert.Equal(t, 3, executionCount, "All tasks should have executed")
	})

	t.Run("No tasks", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		errors := g.Wait()
		assert.Empty(t, errors, "Should have no errors when no tasks are added")
	})
}

func TestFaultTolerantGroup_WaitWithResults(t *testing.T) {
	t.Run("Track individual task results", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		g.Go(func() error {
			return nil // task 0: success
		})
		g.Go(func() error {
			return errors.New("task 1 failed")
		})
		g.Go(func() error {
			return nil // task 2: success
		})
		g.Go(func() error {
			return errors.New("task 3 failed")
		})

		results := g.WaitWithResults()
		assert.Len(t, results, 2, "Should have 2 failed tasks")
		assert.Nil(t, results[0], "Task 0 should have no error")
		assert.Equal(t, "task 1 failed", results[1].Error())
		assert.Nil(t, results[2], "Task 2 should have no error")
		assert.Equal(t, "task 3 failed", results[3].Error())
	})

	t.Run("All tasks succeed with WaitWithResults", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		g.Go(func() error { return nil })
		g.Go(func() error { return nil })
		g.Go(func() error { return nil })

		results := g.WaitWithResults()
		assert.Empty(t, results, "Should have no errors in results map when all tasks succeed")
	})
}

func TestFaultTolerantGroup_ConcurrentSafety(t *testing.T) {
	t.Run("Concurrent task addition", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		// Add tasks concurrently
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(index int) {
				g.Go(func() error {
					if index%2 == 0 {
						return fmt.Errorf("error from task %d", index)
					}
					return nil
				})
				done <- true
			}(i)
		}

		// Wait for all tasks to be added
		for i := 0; i < 10; i++ {
			<-done
		}

		errors := g.Wait()
		assert.Len(t, errors, 5, "Should have 5 errors from even-numbered tasks")
	})

	t.Run("Large number of tasks", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		const numTasks = 100
		for i := 0; i < numTasks; i++ {
			g.Go(func() error {
				return nil
			})
		}

		errors := g.Wait()
		assert.Empty(t, errors, "Should have no errors for successful tasks")
	})
}

func TestFaultTolerantGroup_ErrorTypes(t *testing.T) {
	t.Run("Different error types", func(t *testing.T) {
		g := errgroup.NewFaultTolerant()

		customError := fmt.Errorf("custom error")
		g.Go(func() error { return customError })
		g.Go(func() error { return errors.New("standard error") })
		g.Go(func() error { return nil })

		errors := g.Wait()
		assert.Len(t, errors, 2, "Should have 2 errors")

		// Check that original error types are preserved
		assert.Equal(t, customError, errors[0])
		assert.Equal(t, "standard error", errors[1].Error())
	})
}
