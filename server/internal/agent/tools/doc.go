// Package tools defines every tool the Lumilio Agent may call. [RegisterAll]
// installs them in the core tool registry: producers that resolve a library
// query into a reference (filter, lookup, peek, combine, dedupe, quality
// filter), readers that describe or inspect a reference, and effects that
// mutate the library only after confirmation (create album, add to album, tag,
// bulk like).
//
// Every reference-producing tool returns a [RefToolOutput]: a receipt or a
// typed recoverable error, never raw asset data.
//
//atlas:group agent
package tools
