package runtimeconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/platform"
	"desktop/internal/state"
)

// journalCrashFixture is a runtime config store with two valid intents: the
// previous generation and a candidate that differs from it, both loadable.
type journalCrashFixture struct {
	t         *testing.T
	paths     platform.Paths
	store     *Store
	previous  string
	candidate string
}

func newJournalCrashFixture(t *testing.T) *journalCrashFixture {
	t.Helper()
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(paths)
	draft, err := store.ReadDraft()
	if err != nil {
		t.Fatal(err)
	}
	settings := draft.Settings
	settings.NetworkMode = "custom"
	settings.Listen = "127.0.0.1:1"
	settings.StoragePath = filepath.Join(t.TempDir(), "previous-storage")
	settings.HardwareAcceleration = "none"
	previousDraft, err := store.PatchDraft(draft.TOML, settings)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := store.Validate(filepath.Join(paths.RuntimeIntents, "previous.toml"), []byte(previousDraft.TOML))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteIntent(previous); err != nil {
		t.Fatal(err)
	}
	settings.StoragePath = filepath.Join(t.TempDir(), "candidate-storage")
	candidateDraft, err := store.PatchDraft(previousDraft.TOML, settings)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := store.Validate(filepath.Join(paths.RuntimeIntents, "candidate.toml"), []byte(candidateDraft.TOML))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteIntent(candidate); err != nil {
		t.Fatal(err)
	}
	if previous.Fingerprint == candidate.Fingerprint {
		t.Fatal("previous and candidate intents share a fingerprint")
	}
	return &journalCrashFixture{t: t, paths: paths, store: store, previous: previous.Fingerprint, candidate: candidate.Fingerprint}
}

// crash leaves the files exactly as the transaction controller has them at
// one journal phase: LKG at previous, current at the given generation, and
// the journal open at the phase.
func (f *journalCrashFixture) crash(phase JournalPhase, current string) {
	f.t.Helper()
	if err := f.store.WritePointer(f.paths.RuntimeLKG, f.previous); err != nil {
		f.t.Fatal(err)
	}
	if err := f.store.WritePointer(f.paths.RuntimeCurrent, current); err != nil {
		f.t.Fatal(err)
	}
	if err := f.store.WriteJournal(Journal{
		OperationID: "apply-crash-" + string(phase), Mode: "apply", Phase: phase,
		PreviousFingerprint: f.previous, CandidateFingerprint: f.candidate,
	}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *journalCrashFixture) pointers() (current, lkg string, journalOpen bool) {
	f.t.Helper()
	currentPointer, err := f.store.CurrentPointer()
	if err != nil {
		f.t.Fatal(err)
	}
	lkgPointer, err := f.store.LastKnownGoodPointer()
	if err != nil {
		f.t.Fatal(err)
	}
	_, statErr := os.Stat(f.paths.RuntimeApply)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		f.t.Fatal(statErr)
	}
	return currentPointer.Fingerprint, lkgPointer.Fingerprint, statErr == nil
}

// Every journal phase a crash can leave behind has one verdict: the previous
// generation (or a candidate the journal recorded as ready) starts on its
// own and the readiness promotion closes the journal; a candidate that was
// selected but never proven ready waits for the user to restore the last
// known-good generation, which leaves current, LKG, and the journal
// coherent. No phase ever starts an unproven candidate or loses the
// previous generation as a rollback target.
func TestEveryApplyCrashPhaseRecoversCoherently(t *testing.T) {
	cases := []struct {
		phase          JournalPhase
		current        string // "previous" or "candidate"
		safeToStart    bool
		needsRollback  bool
		startsWith     string // generation running after recovery
		pointerInvalid bool   // the placement cannot come from the controller
	}{
		{phase: PhasePrepared, current: "previous", safeToStart: true, startsWith: "previous"},
		{phase: PhasePrepared, current: "candidate", pointerInvalid: true},
		{phase: PhaseStoppingPrevious, current: "previous", safeToStart: true, startsWith: "previous"},
		{phase: PhasePreviousStopped, current: "previous", safeToStart: true, startsWith: "previous"},
		{phase: PhasePreviousStopped, current: "candidate", needsRollback: true, startsWith: "previous"},
		{phase: PhaseCandidateSelected, current: "candidate", needsRollback: true, startsWith: "previous"},
		{phase: PhaseStoppingCandidate, current: "candidate", needsRollback: true, startsWith: "previous"},
		{phase: PhaseStoppingCandidate, current: "previous", needsRollback: true, startsWith: "previous"},
		{phase: PhaseRollbackSelected, current: "previous", safeToStart: true, startsWith: "previous"},
		{phase: PhaseRollbackSelected, current: "candidate", pointerInvalid: true},
		{phase: PhaseCommitting, current: "candidate", safeToStart: true, startsWith: "candidate"},
		{phase: PhaseCommitting, current: "previous", pointerInvalid: true},
	}
	for _, tc := range cases {
		t.Run(string(tc.phase)+"/"+tc.current, func(t *testing.T) {
			f := newJournalCrashFixture(t)
			current := f.previous
			if tc.current == "candidate" {
				current = f.candidate
			}
			f.crash(tc.phase, current)

			// The Desktop restarts: a fresh store on the same files.
			restarted := NewStore(f.paths)
			result, err := restarted.Reconcile()
			if tc.pointerInvalid {
				if err == nil {
					t.Fatalf("phase %s with current=%s was accepted: %+v", tc.phase, tc.current, result)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.SafeToStart != tc.safeToStart || result.NeedsRollback != tc.needsRollback || result.NeedsResume == tc.needsRollback {
				t.Fatalf("verdict for %s/%s = %+v", tc.phase, tc.current, result)
			}
			if result.LastKnownGood.Fingerprint != f.previous {
				t.Fatalf("the previous generation stopped being the rollback target: %+v", result)
			}

			if tc.needsRollback {
				// The user restores the last known-good generation through
				// the same controller the Settings window drives, on a
				// stopped runtime.
				snapshots := state.NewWithInstanceID("journal-crash")
				controller := NewTransactionController(restarted, snapshots, operation.New(), &fakeLifecycle{store: snapshots})
				if _, err := controller.RestoreLastKnownGood("restore-"+string(tc.phase), 0); err != nil {
					t.Fatalf("restore last known-good: %v", err)
				}
			} else {
				// The runtime starts the current generation and reaches the
				// readiness boundary, which promotes it and closes the journal.
				if err := restarted.PromoteCurrentToLastKnownGood(); err != nil {
					t.Fatalf("readiness promotion: %v", err)
				}
			}
			currentAfter, lkgAfter, journalOpen := f.pointers()
			want := f.previous
			if tc.startsWith == "candidate" {
				want = f.candidate
			}
			if currentAfter != want || lkgAfter != want || journalOpen {
				t.Fatalf("after recovery current=%s lkg=%s journal=%v, want both %s and no journal", currentAfter, lkgAfter, journalOpen, tc.startsWith)
			}
			if _, err := restarted.LoadCurrentConfig(); err != nil {
				t.Fatalf("recovered configuration does not load: %v", err)
			}
			if again, err := restarted.Reconcile(); err != nil || again.NeedsResume || again.NeedsRollback {
				t.Fatalf("a second restart still sees work: %+v err=%v", again, err)
			}
		})
	}
}

// A restore is refused while a runtime still owns its generation, so the
// pointer never changes under a process that is reading it.
func TestRestoreLastKnownGoodWaitsForOwnershipRelease(t *testing.T) {
	f := newJournalCrashFixture(t)
	f.crash(PhaseCandidateSelected, f.candidate)
	snapshots := state.NewWithInstanceID("journal-crash-owned")
	snapshots.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Runtime.Phase = dto.RuntimeRunning
		snapshot.Runtime.Ownership = dto.OwnershipHeld
	})
	controller := NewTransactionController(NewStore(f.paths), snapshots, operation.New(), &fakeLifecycle{store: snapshots})
	_, err := controller.RestoreLastKnownGood("restore-owned", 0)
	if operation.ErrorCodeOf(err) != dto.ErrorStopTimeout {
		t.Fatalf("restore under held ownership = %v, code %q", err, operation.ErrorCodeOf(err))
	}
	current, lkg, journalOpen := f.pointers()
	if current != f.candidate || lkg != f.previous || !journalOpen {
		t.Fatalf("a refused restore changed the files: current=%s lkg=%s journal=%v", current, lkg, journalOpen)
	}
}
