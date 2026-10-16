// Package locations resolves logical Assets to a present physical file
// immediately before media I/O. It holds the RepositoryFS lifecycle lease for
// the lifetime of the returned capability and falls through unavailable
// copies without changing catalog state.
//
//atlas:group storage
package locations
