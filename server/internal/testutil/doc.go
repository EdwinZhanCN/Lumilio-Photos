// Package testutil seeds catalog fixtures for tests. [InsertAssetOccurrence]
// creates one normalized Asset with its repository entry through any
// [SQLExecutor], so tests exercise real catalog invariants rather than
// hand-written rows. It is imported only by _test.go files.
//
//atlas:group foundation
package testutil
