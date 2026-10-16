// Package marker owns typed portable-marker readings and guarded atomic
// replacement. Observation uses only [Reader]; [WriteAtomic] is a separate
// mutation primitive, called under the caller's identity/ownership barrier.
// A guard is rechecked just before rename; advisory ownership cannot prevent
// an unrelated process from replacing a file after that check.
//
//atlas:group storage
package marker
