// Package inject materializes ask-time context and mention bindings into the
// session ref ledger and a fixed-schema, explicitly untrusted data message.
// Asset data never crosses the LLM boundary (INV-1, INV-7).
//
//atlas:group agent
package inject
