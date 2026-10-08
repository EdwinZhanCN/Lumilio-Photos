// Package problem is the closed RFC 9457 vocabulary of the API. Every non-2xx
// JSON response below /api/v1 is a Problem with application/problem+json that
// carries only type, status, and an opaque instance plus the fields of an
// exact registered subtype ([Registered], [Descriptor]). It never carries a
// title, detail, display copy, or the private Go cause. Generic status-only
// failures use about:blank ([About]); a Lumilio type exists only when the
// client needs a distinct explanation, recovery path, or machine action.
//
// Handlers choose a typed [Failure] and hand its cause to the single writer
// [server/internal/api.WriteProblem], which mints the occurrence instance and
// logs it with the cause on the request log, so a public instance identifies
// exactly one log event without revealing the cause. Failures after a request
// was accepted (upload and agent streams, scan, restore, cloud, native storage
// tasks) persist a transport-neutral [Reference] instead.
//
// `task dto` runs tools/openapinormalize (exact discriminator unions and
// problem media types) and tools/problemcatalog (the published pages under
// site/docs/public/problems). `task architecture:check` rejects legacy
// responders, direct non-success JSON, raw public error text, unregistered or
// undocumented types, and catalog drift.
//
// It is a leaf package: handlers and transports depend on these descriptors,
// while domain packages stay unaware of HTTP problem URIs.
//
//atlas:group http
package problem
