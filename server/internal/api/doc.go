// Package api is the HTTP transport root. [NewRouter] maps every route to a
// controller interface and applies the authentication, setup, origin, and
// rate-limit boundaries; [RegisterSPA] serves the built Web application.
//
// Responses are JSON ([JSONOK]) or RFC 9457 Problems ([WriteProblem]); the
// Problem catalog lives in [server/internal/api/problem]. Handlers live in
// [server/internal/api/handler] and wire types in [server/internal/api/dto].
// OpenAPI is generated from handler annotations and is the contract the Web
// client is typed against.
//
//atlas:group http
package api
