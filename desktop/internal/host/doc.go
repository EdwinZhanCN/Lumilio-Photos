// Package host adapts the Desktop control plane to MyGo. It is the only
// package that imports the framework's application API.
//
// [Host] owns what the tray and the Settings window share: the navigation
// guard that keeps recovery and onboarding from being left early, opening
// the product, and the coordinated quit and relaunch. [Host.Run] starts the
// application: a menu-bar app without a Dock icon that outlives its windows.
// The Settings window is created on demand and destroyed when closed, so
// nothing is kept in memory for a window nobody looks at; its state lives in
// [desktop/internal/panel.Panel], which outlives the window.
//
// [Tray] is the menu-bar icon and its native menu. A quit from the system
// goes through the same shutdown coordinator as the tray's, and an installed
// update is applied by the same quiesce followed by a relaunch.
//
// Runtime and Lumen controllers remain the lifecycle owners of their
// processes. The host gives them the system's folder picker and file
// manager, and gives the update controller the framework's signed updater.
//
//atlas:group desktop-host
package host
