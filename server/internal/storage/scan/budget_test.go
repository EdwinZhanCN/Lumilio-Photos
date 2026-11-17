package scan

import (
	"fmt"
	"testing"
)

// writerRowBudget is the budget #222 invariant 6 states, independent of the
// scanner's own constant, so raising MaxBatchRows cannot move the goalposts.
const writerRowBudget = 256

// #222 invariant 6, the structural writer budget: no scan writer transaction
// writes more than 256 entry rows, measured by SQLite's own count of the
// rows each transaction wrote. One directory of 1,500 files makes a single
// diff, and later a single sweep, larger than any batch may be.
func TestWriterTransactionsStayWithinTheRowBudget(t *testing.T) {
	if MaxBatchRows > writerRowBudget {
		t.Fatalf("MaxBatchRows = %d exceeds the %d-row budget", MaxBatchRows, writerRowBudget)
	}
	f := newFixture(t, 0)
	const files = 1500
	for index := 0; index < files; index++ {
		f.write(f.primary, fmt.Sprintf("bulk/f%04d.jpg", index), fmt.Sprintf("bulk-%d", index))
	}
	// Start the adaptive sizer at the cap, so the budget is tested where the
	// cap binds rather than where a slow disk already keeps batches small.
	f.scanner.batch.rows = MaxBatchRows

	f.resetObservations()
	if scan := f.scan(f.primary); scan.Status != StatusCompleted || scan.NewEntries != files+1 {
		t.Fatalf("first scan = %+v, want %d new entries", scan, files+1)
	}
	if largest := f.writer.maxRowsPerTx(); largest > writerRowBudget || largest < minBatchRows {
		t.Fatalf("walk transactions wrote up to %d rows, want batches between %d and %d", largest, minBatchRows, writerRowBudget)
	}

	f.resetObservations()
	if bound := f.hash(f.primary); bound != files {
		t.Fatalf("hashed %d files, want %d", bound, files)
	}
	if largest := f.writer.maxRowsPerTx(); largest > writerRowBudget {
		t.Fatalf("hash transactions wrote up to %d rows, budget %d", largest, writerRowBudget)
	}

	f.remove(f.primary, "bulk")
	f.scanner.batch.rows = MaxBatchRows
	f.resetObservations()
	if scan := f.scan(f.primary); scan.Status != StatusCompleted || scan.Missing != files {
		t.Fatalf("scan after removal = %+v, want %d missing", scan, files)
	}
	if largest := f.writer.maxRowsPerTx(); largest > writerRowBudget || largest < minBatchRows {
		t.Fatalf("sweep transactions wrote up to %d rows, want pages between %d and %d", largest, minBatchRows, writerRowBudget)
	}
	if writes := f.entryWrites(); len(writes) != files+1 {
		t.Fatalf("sweep wrote %d entry rows, want %d files marked missing and one directory removed", len(writes), files+1)
	}
}
