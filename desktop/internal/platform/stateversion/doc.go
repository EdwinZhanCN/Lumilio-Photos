// Package stateversion checks the schema version of a persisted Desktop state
// file. Version 1 of every file is the v26.1.0-rc.1 compatibility baseline.
// A reader that changes its file's version handles each older supported
// version before calling Check, so later builds read older files forward;
// a file from a newer Desktop is rejected rather than misread.
//
//atlas:group desktop-platform
package stateversion
