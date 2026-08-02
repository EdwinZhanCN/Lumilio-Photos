// Package operation tracks every durable or lifecycle mutation exposed by the
// Desktop control plane. It owns request-id idempotency and one mutation gate
// per aggregate; controllers remain responsible for their actor state.
package operation

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"desktop/internal/control/dto"
)

type State string

const (
	Accepted  State = "accepted"
	Running   State = "running"
	Succeeded State = "succeeded"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

var ErrClosed = errors.New("operation registry is closed")

type ControlError struct {
	DTO dto.Error
}

func (e *ControlError) Error() string {
	if e == nil {
		return ""
	}
	if e.DTO.Message == "" {
		return string(e.DTO.Code)
	}
	return e.DTO.Message
}

func (e *ControlError) Unwrap() error { return nil }

func NewError(code dto.ErrorCode, message string) *ControlError {
	return &ControlError{DTO: dto.Error{Code: code, Message: message}}
}

func WithOperation(err error, operationID string) error {
	if err == nil {
		return nil
	}
	var controlErr *ControlError
	if errors.As(err, &controlErr) {
		copy := *controlErr
		copy.DTO.OperationID = operationID
		return &copy
	}
	return &ControlError{DTO: dto.Error{
		Code:        dto.ErrorRecoveryRequired,
		Message:     err.Error(),
		OperationID: operationID,
	}}
}

func ErrorCodeOf(err error) dto.ErrorCode {
	var controlErr *ControlError
	if errors.As(err, &controlErr) {
		return controlErr.DTO.Code
	}
	return ""
}

type record struct {
	receipt     dto.OperationReceipt
	state       State
	cancellable bool
	err         dto.Error
}

type Registry struct {
	mu         sync.Mutex
	sequence   atomic.Uint64
	operations map[string]record
	byRequest  map[string]string
	active     map[string]string
	closed     bool
}

func New() *Registry {
	return &Registry{
		operations: make(map[string]record),
		byRequest:  make(map[string]string),
		active:     make(map[string]string),
	}
}

// Accept reserves an aggregate without waiting. A repeated request ID returns
// the original receipt, even after completion, so frontend retries are safe.
func (r *Registry) Accept(requestID, aggregate string, acceptedVersion uint64, cancellable bool) (dto.OperationReceipt, error) {
	requestID = strings.TrimSpace(requestID)
	aggregate = strings.TrimSpace(aggregate)
	if requestID == "" || aggregate == "" {
		return dto.OperationReceipt{}, NewError(dto.ErrorInvalidArgument, "requestID and aggregate are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return dto.OperationReceipt{}, ErrClosed
	}
	if operationID, ok := r.byRequest[requestID]; ok {
		return r.operations[operationID].receipt, nil
	}
	if _, ok := r.active[aggregate]; ok {
		return dto.OperationReceipt{}, NewError(dto.ErrorOperationConflict, fmt.Sprintf("%s already has an active operation", aggregate))
	}

	operationID := fmt.Sprintf("op-%d", r.sequence.Add(1))
	receipt := dto.OperationReceipt{
		OperationID:     operationID,
		RequestID:       requestID,
		Aggregate:       aggregate,
		AcceptedVersion: acceptedVersion,
	}
	r.operations[operationID] = record{
		receipt:     receipt,
		state:       Accepted,
		cancellable: cancellable,
	}
	r.byRequest[requestID] = operationID
	r.active[aggregate] = operationID
	return receipt, nil
}

func (r *Registry) MarkRunning(operationID string) error {
	return r.update(operationID, func(item *record) error {
		if item.state != Accepted {
			return fmt.Errorf("operation %s is not accepted", operationID)
		}
		item.state = Running
		return nil
	})
}

func (r *Registry) Succeed(operationID string) error {
	return r.finish(operationID, Succeeded, dto.Error{})
}

func (r *Registry) Cancel(operationID string, err error) error {
	return r.finish(operationID, Cancelled, errorDTO(err, operationID))
}

func (r *Registry) Fail(operationID string, err error) error {
	return r.finish(operationID, Failed, errorDTO(err, operationID))
}

func (r *Registry) Get(operationID string) (dto.OperationSnapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.operations[operationID]
	if !ok {
		return dto.OperationSnapshot{}, false
	}
	return snapshotOf(item), true
}

func (r *Registry) Snapshot() []dto.OperationSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]dto.OperationSnapshot, 0, len(r.operations))
	for _, item := range r.operations {
		result = append(result, snapshotOf(item))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].OperationID < result[j].OperationID
	})
	return result
}

func (r *Registry) Close() {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
}

func (r *Registry) update(operationID string, update func(*record) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.operations[operationID]
	if !ok {
		return NewError(dto.ErrorInvalidArgument, "unknown operation")
	}
	if err := update(&item); err != nil {
		return err
	}
	r.operations[operationID] = item
	return nil
}

func (r *Registry) finish(operationID string, state State, itemError dto.Error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.operations[operationID]
	if !ok {
		return NewError(dto.ErrorInvalidArgument, "unknown operation")
	}
	if item.state != Accepted && item.state != Running {
		return fmt.Errorf("operation %s is already complete", operationID)
	}
	item.state = state
	item.err = itemError
	r.operations[operationID] = item
	delete(r.active, item.receipt.Aggregate)
	return nil
}

func snapshotOf(item record) dto.OperationSnapshot {
	return dto.OperationSnapshot{
		OperationID: item.receipt.OperationID,
		RequestID:   item.receipt.RequestID,
		Aggregate:   item.receipt.Aggregate,
		State:       string(item.state),
		Cancellable: item.cancellable,
		Error:       item.err,
	}
}

func errorDTO(err error, operationID string) dto.Error {
	if err == nil {
		return dto.Error{OperationID: operationID}
	}
	var controlErr *ControlError
	if errors.As(err, &controlErr) {
		result := controlErr.DTO
		result.OperationID = operationID
		return result
	}
	return dto.Error{Code: dto.ErrorRecoveryRequired, Message: err.Error(), OperationID: operationID}
}
