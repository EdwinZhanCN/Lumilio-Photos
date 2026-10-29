// Package runtime owns the in-process Server generation. Its actor is the
// only writer of lifecycle state and generation ownership; bindings enqueue
// bounded, non-blocking commands and receive receipts immediately.
//
//atlas:group desktop-runtime
package runtime
