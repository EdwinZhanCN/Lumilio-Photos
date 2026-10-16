// Package dto defines the Desktop control-plane wire types. [DesktopSnapshot]
// is the single revisioned state the Settings window renders, composed of
// [RuntimeSnapshot], [LumenSnapshot], [UpdateSnapshot], [ShutdownSnapshot],
// storage and host summaries, and in-flight [OperationSnapshot] values.
//
// Phase enums ([RuntimePhase], [LumenInstallPhase], [LumenProcessPhase],
// [ShutdownPhase], [DesiredState]) are the lifecycle vocabulary every
// controller reports in. [InitialSnapshot] is the state before any controller
// starts.
//
//atlas:group desktop-control
package dto
