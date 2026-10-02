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
//atlas:group http
package handler
