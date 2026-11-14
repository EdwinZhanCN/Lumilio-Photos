package panel

import (
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/locales"
	"desktop/internal/operation"

	"github.com/egoist/mygo/ui"
)

type draftPhase int

const (
	phaseLoading draftPhase = iota
	phaseSaved
	phasePreparing
	phaseDraft
	phaseSaving
	phaseSuccess
	phaseError
)

// savedHold is how long the save bar keeps saying that the draft was saved.
const savedHold = 1800 * time.Millisecond

// draft is the one editable copy of the runtime configuration and the
// Desktop preferences. Reading or editing it persists nothing: only a
// successful save changes the current fingerprint.
type draft struct {
	runtimeDraft *dto.RuntimeConfigDraft
	runtime      *dto.RuntimeConfigSettings
	preferences  *dto.DesktopPreferences
	saved        *dto.DesktopPreferences
	validation   *dto.ConfigValidation

	runtimeDirty    bool
	preferenceDirty bool
	phase           draftPhase
	err             string

	// candidate is the authoritative TOML the settings are projected from.
	candidate string
	// A later patch or load supersedes the answers of earlier ones.
	patchSequence  int
	loadSequence   int
	loadedInstance string
	// savedStoragePath is the Default Storage Location of a saved
	// configuration. Choosing another folder then is a move, which is
	// confirmed; on the first launch the path is only the wizard's proposal.
	savedStoragePath string
	successUntil     time.Time
}

func (d *draft) dirty() bool { return d.runtimeDirty || d.preferenceDirty }

func (d *draft) busy() bool { return d.phase == phasePreparing || d.phase == phaseSaving }

func (d *draft) settle(now time.Time) time.Time {
	if d.phase != phaseSuccess {
		return time.Time{}
	}
	if !now.Before(d.successUntil) {
		d.phase = phaseSaved
		return time.Time{}
	}
	return d.successUntil
}

func (p *Panel) loadDraft() {
	d := &p.draft
	d.phase, d.err = phaseLoading, ""
	d.patchSequence++
	d.loadSequence++
	sequence := d.loadSequence
	p.async(func() func() {
		next, err := p.services.Runtime.ReadConfigDraft()
		return func() {
			if sequence != d.loadSequence {
				return
			}
			if err != nil {
				d.err, d.phase = errorText(err), phaseError
				p.toastError(locales.ToastSettingsLoadFailed(), d.err)
				return
			}
			p.installRuntimeDraft(next)
			p.installPreferences(p.snapshot.Host.Preferences)
		}
	})
}

func (p *Panel) installRuntimeDraft(next dto.RuntimeConfigDraft) {
	d := &p.draft
	settings, validation := next.Settings, next.Validation
	d.runtimeDraft, d.runtime, d.validation = &next, &settings, &validation
	d.candidate = next.TOML
	firstRun := next.Source == "default"
	d.runtimeDirty, d.err = firstRun, ""
	d.phase, d.savedStoragePath = phaseSaved, settings.StoragePath
	if firstRun {
		d.phase, d.savedStoragePath = phaseDraft, ""
	}
}

func (p *Panel) installPreferences(saved dto.DesktopPreferences) {
	d := &p.draft
	current, baseline := saved, saved
	d.preferences, d.saved, d.preferenceDirty = &current, &baseline, false
	p.apply(saved)
}

// updateRuntime edits the projected settings. Go patches the authoritative
// TOML candidate and validates it with the real strict Server loader.
func (p *Panel) updateRuntime(mutate func(*dto.RuntimeConfigSettings)) {
	d := &p.draft
	if d.runtime == nil {
		return
	}
	next := *d.runtime
	mutate(&next)
	if next == *d.runtime {
		return
	}
	d.runtime, d.runtimeDirty = &next, true
	d.phase, d.err = phasePreparing, ""
	d.patchSequence++
	sequence, candidate := d.patchSequence, d.candidate
	p.async(func() func() {
		patched, err := p.services.Runtime.PatchConfigDraft(candidate, next)
		return func() {
			if sequence != d.patchSequence {
				return
			}
			if err != nil {
				d.err, d.phase = errorText(err), phaseError
				d.validation = &dto.ConfigValidation{Issues: []dto.ConfigIssue{{Message: d.err}}}
				return
			}
			if d.runtimeDraft != nil {
				patched.BaseFingerprint = d.runtimeDraft.BaseFingerprint
			}
			settings, validation := patched.Settings, patched.Validation
			d.runtimeDraft, d.runtime, d.validation = &patched, &settings, &validation
			d.candidate, d.phase = patched.TOML, phaseDraft
		}
	})
}

// updatePreference edits the Desktop preferences and previews them at once.
// Cancel reads the saved ones again, so the preview has one save boundary.
func (p *Panel) updatePreference(mutate func(*dto.DesktopPreferences)) {
	d := &p.draft
	if d.preferences == nil {
		return
	}
	next := *d.preferences
	mutate(&next)
	d.preferences = &next
	d.preferenceDirty = d.saved == nil || !preferencesEqual(next, *d.saved)
	if !d.busy() {
		d.phase = phaseDraft
	}
	d.err = ""
	p.apply(next)
}

func (p *Panel) chooseDefaultStorage() {
	d := &p.draft
	title := locales.StorageChooseDefaultLocation()
	p.async(func() func() {
		path, err := p.services.Storage.PickLocation(title)
		return func() {
			if err != nil {
				if operation.ErrorCodeOf(err) != dto.ErrorSelectionCancelled {
					p.toastError(locales.ToastStoragePickFailed(), errorText(err))
				}
				return
			}
			if path == "" || d.runtime == nil || path == d.runtime.StoragePath {
				return
			}
			if d.savedStoragePath != "" {
				p.pendingStoragePath, p.confirmStorageMove = path, true
				return
			}
			p.updateRuntime(func(settings *dto.RuntimeConfigSettings) { settings.StoragePath = path })
		}
	})
}

// storageMoveConfirmation asks before the saved Default Storage Location is
// pointed at another folder: the person moves the data, Lumilio only
// verifies the identity it finds there.
func (p *Panel) storageMoveConfirmation(c *ui.Context) {
	choice := ui.AlertDialog(c, &p.confirmStorageMove,
		locales.StorageChooseDefaultLocation(), locales.StorageDefaultLocationMigrationConfirmation(),
		locales.CommonCancel(), locales.CommonContinue())
	if choice == 1 {
		path := p.pendingStoragePath
		p.updateRuntime(func(settings *dto.RuntimeConfigSettings) { settings.StoragePath = path })
	}
	if choice >= 0 || choice == ui.AlertDismissed {
		p.pendingStoragePath = ""
	}
}

func (p *Panel) cancelDraft() {
	p.draft.patchSequence++
	p.loadDraft()
}

// saveDraft saves the preferences, then runs the configuration transaction:
// an apply with a controlled restart while the Server runs, a plain save
// otherwise. done hears how it ended.
func (p *Panel) saveDraft(done func(ok bool)) {
	d := &p.draft
	finish := func(ok bool, message string) {
		if ok {
			d.preferenceDirty, d.runtimeDirty = false, false
			d.phase, d.successUntil = phaseSuccess, p.now().Add(savedHold)
		} else {
			d.err, d.phase = message, phaseError
		}
		if done != nil {
			done(ok)
		}
	}
	if !p.hasSnapshot || d.runtimeDraft == nil || d.preferences == nil || d.busy() {
		if done != nil {
			done(false)
		}
		return
	}
	d.phase, d.err = phaseSaving, ""

	preferences, preferenceDirty, runtimeDirty := *d.preferences, d.preferenceDirty, d.runtimeDirty
	candidate, base := d.candidate, d.runtimeDraft.BaseFingerprint
	runtime := p.snapshot.Runtime
	running := runtime.Phase == dto.RuntimeRunning && runtime.Ownership == dto.OwnershipHeld
	requestID := p.requestID("config")

	p.async(func() func() {
		saved := preferences
		if preferenceDirty {
			var err error
			if saved, err = p.services.Desktop.SavePreferences(preferences); err != nil {
				return func() { finish(false, errorText(err)) }
			}
		}
		if !runtimeDirty {
			return func() {
				p.installPreferences(saved)
				finish(true, "")
			}
		}
		checked, err := p.services.Runtime.ValidateConfig(candidate)
		if err != nil {
			return func() {
				p.installPreferences(saved)
				finish(false, errorText(err))
			}
		}
		if !checked.Valid {
			return func() {
				p.installPreferences(saved)
				d.validation = &checked
				finish(false, firstIssue(&checked))
			}
		}
		var receipt dto.OperationReceipt
		if running {
			receipt, err = p.services.Runtime.ApplyConfig(requestID, runtime.Version, base, candidate)
		} else {
			receipt, err = p.services.Runtime.SaveConfig(requestID, runtime.Version, base, candidate)
		}
		if err != nil {
			return func() {
				p.installPreferences(saved)
				finish(false, errorText(err))
			}
		}
		return func() {
			p.installPreferences(saved)
			d.validation = &checked
			p.await(receipt, func(message string, ok bool) {
				if !ok {
					finish(false, message)
					return
				}
				p.async(func() func() {
					next, err := p.services.Runtime.ReadConfigDraft()
					return func() {
						if err != nil {
							finish(false, errorText(err))
							return
						}
						p.installRuntimeDraft(next)
						finish(true, "")
					}
				})
			})
		}
	})
}

func preferencesEqual(left, right dto.DesktopPreferences) bool {
	left.Version, right.Version = 0, 0
	return left == right
}
