//go:build !darwin

package supervisor

// stripQuarantine is a no-op off macOS, where there is no Gatekeeper quarantine
// attribute to clear.
func stripQuarantine(resourcesDir string) error { return nil }
