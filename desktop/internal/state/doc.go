// Package state owns the immutable DesktopSnapshot and its latest-only
// notification stream. Producers commit reducers; consumers always read the
// complete snapshot after receiving a notice.
//
//atlas:group desktop-control
package state
