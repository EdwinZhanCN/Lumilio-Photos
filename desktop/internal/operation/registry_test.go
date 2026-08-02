package operation

import (
	"testing"

	"desktop/internal/control/dto"
)

func TestRegistryIsIdempotentAndSerializesAggregates(t *testing.T) {
	registry := New()
	first, err := registry.Accept("request-1", "runtime", 4, true)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := registry.Accept("request-1", "runtime", 99, true)
	if err != nil {
		t.Fatal(err)
	}
	if retry != first {
		t.Fatalf("retry receipt changed: %#v != %#v", retry, first)
	}
	if _, err := registry.Accept("request-2", "runtime", 4, true); ErrorCodeOf(err) != dto.ErrorOperationConflict {
		t.Fatalf("expected operation conflict, got %v", err)
	}
	if err := registry.MarkRunning(first.OperationID); err != nil {
		t.Fatal(err)
	}
	if err := registry.Succeed(first.OperationID); err != nil {
		t.Fatal(err)
	}
	second, err := registry.Accept("request-2", "runtime", 4, true)
	if err != nil {
		t.Fatal(err)
	}
	if second.OperationID == first.OperationID {
		t.Fatal("completed aggregate was not released")
	}
}

func TestRegistryPublishesStableFailure(t *testing.T) {
	registry := New()
	receipt, err := registry.Accept("request", "runtime", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Fail(receipt.OperationID, NewError(dto.ErrorReadinessTimeout, "server did not become ready")); err != nil {
		t.Fatal(err)
	}
	item, ok := registry.Get(receipt.OperationID)
	if !ok || item.State != string(Failed) || item.Error.Code != dto.ErrorReadinessTimeout {
		t.Fatalf("unexpected operation: %+v", item)
	}
}
