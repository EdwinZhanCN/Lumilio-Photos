// Package platform isolates Desktop operating-system differences:
// application-support and cache paths ([ResolvePaths]), bundled resources and
// media tools ([ResolveBundledResourcesDir], [ResolveMediaToolPaths]), libvips
// module configuration, crash-safe atomic file replacement ([WriteAtomic]),
// and owner-only directories ([EnsurePrivateDirectory]).
//
//atlas:group desktop-platform
package platform
