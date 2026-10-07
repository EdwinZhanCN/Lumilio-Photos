// Package dbtypes holds the value types the catalog persists in JSON and text
// columns: [AssetType], per-type metadata ([PhotoSpecificMetadata],
// [VideoSpecificMetadata], [AudioSpecificMetadata]), ML result payloads
// (faces, OCR, classification), [StackKind], vectors, and nullable collection
// types.
//
// It has no dependencies so every layer, including media utilities, can speak
// catalog vocabulary without importing the database.
//
//atlas:group foundation
package dbtypes
