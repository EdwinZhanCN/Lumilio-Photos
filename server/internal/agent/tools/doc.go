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
// read_ocr reads authoritative OCR rows through the owner-bound authorized
// library for a reference of at most two Assets; search_text uses the Bleve
// sidecar only to produce a matching reference. OCR output restores reference
// and provider order, separates unsupported media from missing and empty
// results, and exposes only sanitized filenames, status, region count, and
// capped text — never UUIDs, paths, confidence, geometry, or model identity.
//
//atlas:group agent
package tools
