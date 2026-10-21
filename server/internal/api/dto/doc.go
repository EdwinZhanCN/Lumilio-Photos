// Package dto defines the HTTP request and response shapes. DTO structs and
// their swag annotations are the OpenAPI source of truth: `task dto`
// regenerates server/docs and web/src/lib/http-commons/schema.d.ts from them.
// Conversions from catalog and service types live beside each DTO so handlers
// stay thin.
//
//atlas:group http
package dto
