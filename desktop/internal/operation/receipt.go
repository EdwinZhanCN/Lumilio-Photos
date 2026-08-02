package operation

import "desktop/internal/control/dto"

func (r *Registry) ReceiptForRequest(requestID string) (dto.OperationReceipt, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	operationID, ok := r.byRequest[requestID]
	if !ok {
		return dto.OperationReceipt{}, false
	}
	item, ok := r.operations[operationID]
	if !ok {
		return dto.OperationReceipt{}, false
	}
	return item.receipt, true
}
