// Package lumen owns the optional Lumen Hub child process. The controller is
// deliberately independent from the Server generation: the Hub is an
// externally supervised process tree, while all UI-facing state still flows
// through the Desktop snapshot.
//
//atlas:group desktop-services
package lumen
