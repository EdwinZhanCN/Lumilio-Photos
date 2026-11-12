package panel

import (
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/locales"
	"desktop/internal/operation"
)

type opState int

const (
	opIdle opState = iota
	opLoading
	opSuccess
	opError
)

// successHold is how long a button keeps saying that its action succeeded.
const successHold = 1200 * time.Millisecond

// tracker follows one control-plane operation for a button. A service call
// only acknowledges the operation; the snapshot's operation list says how it
// ended.
type tracker struct {
	operationID string
	state       opState
	err         string
	resetAt     time.Time
	onSucceeded func()
	onFailed    func(message string)
}

func (t *tracker) loading() bool { return t.state == opLoading }

// begin shows the action as running before its call returns.
func (t *tracker) begin() {
	t.operationID, t.err, t.resetAt = "", "", time.Time{}
	t.state = opLoading
}

// resolve takes the answer of the call begin announced.
func (t *tracker) resolve(receipt dto.OperationReceipt, err error, operations []dto.OperationSnapshot, now time.Time) {
	if err != nil {
		t.reject(errorText(err))
		return
	}
	t.operationID = receipt.OperationID
	t.observe(operations, now)
}

func (t *tracker) reject(message string) {
	t.operationID, t.err, t.state = "", message, opError
	if t.onFailed != nil {
		t.onFailed(message)
	}
}

func (t *tracker) reset() {
	t.operationID, t.err, t.resetAt = "", "", time.Time{}
	t.state = opIdle
}

func (t *tracker) observe(operations []dto.OperationSnapshot, now time.Time) {
	if t.operationID == "" {
		return
	}
	for _, item := range operations {
		if item.OperationID != t.operationID {
			continue
		}
		switch operation.State(item.State) {
		case operation.Accepted, operation.Running:
			t.state = opLoading
		case operation.Succeeded:
			t.operationID, t.err = "", ""
			t.state, t.resetAt = opSuccess, now.Add(successHold)
			if t.onSucceeded != nil {
				t.onSucceeded()
			}
		case operation.Failed, operation.Cancelled:
			t.reject(operationFailure(item))
		}
		return
	}
}

// settle returns the button to idle once its success has shown long enough,
// and reports when that is while it has not.
func (t *tracker) settle(now time.Time) time.Time {
	if t.state != opSuccess {
		return time.Time{}
	}
	if !now.Before(t.resetAt) {
		t.state, t.resetAt = opIdle, time.Time{}
		return time.Time{}
	}
	return t.resetAt
}

// operationTimeout bounds how long the draft waits for a configuration
// transaction, which restarts the Server.
const operationTimeout = 2 * time.Minute

// waiter continues a multi-step flow once an operation ends.
type waiter struct {
	operationID string
	deadline    time.Time
	done        func(message string, ok bool)
}

func (p *Panel) await(receipt dto.OperationReceipt, done func(message string, ok bool)) {
	p.waiters = append(p.waiters, &waiter{
		operationID: receipt.OperationID, deadline: p.now().Add(operationTimeout), done: done,
	})
	p.observeWaiters(p.now())
}

func (p *Panel) observeWaiters(now time.Time) {
	if len(p.waiters) == 0 {
		return
	}
	pending := p.waiters
	p.waiters = nil
	var finished []func()
	for _, w := range pending {
		message, ok, ended := waiterOutcome(w, p.snapshot.Operations, now)
		if !ended {
			p.waiters = append(p.waiters, w)
			continue
		}
		finished = append(finished, func() { w.done(message, ok) })
	}
	// Continuations run once the list is settled: one may await again.
	for _, fn := range finished {
		fn()
	}
}

func waiterOutcome(w *waiter, operations []dto.OperationSnapshot, now time.Time) (message string, ok, ended bool) {
	for _, item := range operations {
		if item.OperationID != w.operationID {
			continue
		}
		switch operation.State(item.State) {
		case operation.Succeeded:
			return "", true, true
		case operation.Failed, operation.Cancelled:
			return operationFailure(item), false, true
		}
	}
	if !now.Before(w.deadline) {
		return locales.ToastSettingsOperationTimeout(), false, true
	}
	return "", false, false
}
