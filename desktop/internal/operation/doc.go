// Package operation tracks every durable or lifecycle mutation exposed by the
// Desktop control plane. It owns request-id idempotency and one mutation gate
// per aggregate; controllers remain responsible for their actor state.
//
//atlas:group desktop-control
package operation
