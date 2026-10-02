// Package handler implements the HTTP controllers that
// [server/internal/api.NewRouter] mounts: one handler type per domain
// ([AssetHandler], [AuthHandler], [AlbumHandler], [StorageHandler],
// [SetupHandler], [AgentHandler], [MusicHandler], [CloudHandler], and others).
//
// Handlers authenticate and authorize the request, bind and validate DTOs,
// call one service, and translate errors into Problems. Upload handlers admit
// batch and chunked transfers into repository staging; long work is accepted
// and tracked, never performed inline.
//
// Browser sessions: access tokens are short-lived Bearer credentials. The
// refresh credential is the host-only HttpOnly lumilio_refresh cookie scoped
// to /api/v1/auth and is never returned in JSON. Each session creation or
// rotation returns a CSRF proof bound to that refresh credential; refresh and
// logout require it in X-CSRF-Token, and GET /api/v1/auth/csrf recovers it for
// an existing cookie session. [AuthHandler] writes every new session through
// one installer so cookie, CSRF, and replacement semantics cannot drift
// between endpoints; requests that create, rotate, or destroy a cookie session
// must match the request origin or the credentialed allowlist.
//
// Storage surface: any authenticated user may list upload targets (GET
// /api/v1/storage/targets) without host paths, capacity, or verification
// detail; every other /api/v1/storage route ([StorageHandler]) is
// administrator-only, including the storage view, repository and Storage
// Location lifecycle, verifications, candidates, native tasks, and
// diagnostics. No storage endpoint deletes user media; detaching is explicit
// (detach-impact, then detach), and creation selects a registered Storage
// Location, never a host path.
//
// Accepted uploads expose their ingest lifecycle at GET
// /api/v1/assets/batch/operations: an upload is complete only when its catalog
// receipt is terminal, not when the multipart request returns. Map points take
// an optional complete south,north,west,east viewport (antimeridian-aware) so
// map rendering stays proportional to the visible area.
//
//atlas:group http
package handler
