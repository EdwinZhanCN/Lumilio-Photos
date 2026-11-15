package catalogtx

import (
	"sync"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

const (
	histogramLowestMicroseconds  int64 = 1
	histogramHighestMicroseconds int64 = int64((2 * time.Minute) / time.Microsecond)
	histogramSignificantFigures        = 3
)

// Recorder maintains fixed-range HDR histograms keyed only by the static
// operation catalog. It never retains individual observations.
type Recorder struct {
	mu                 sync.Mutex
	operations         map[Operation]*operationHistogram
	statements         map[Operation]*statementHistogram
	intervalOperations map[Operation]*operationHistogram
	intervalStatements map[Operation]*statementHistogram
	rejectedSamples    int64
	intervalRejected   int64
}

func NewRecorder() *Recorder {
	return &Recorder{
		operations:         make(map[Operation]*operationHistogram),
		statements:         make(map[Operation]*statementHistogram),
		intervalOperations: make(map[Operation]*operationHistogram),
		intervalStatements: make(map[Operation]*statementHistogram),
	}
}

func (r *Recorder) ObserveTransaction(sample TransactionSample) {
	if r == nil {
		return
	}
	descriptor, ok := sample.Operation.Descriptor()
	if !ok || descriptor.Kind == OperationKindStatement || sample.Role != descriptor.Role || sample.OperationName != "" && sample.OperationName != descriptor.Name {
		r.mu.Lock()
		r.rejectedSamples++
		r.intervalRejected++
		r.mu.Unlock()
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	recordTransactionHistogram(operationHistogramFor(r.operations, sample.Operation), sample)
	recordTransactionHistogram(operationHistogramFor(r.intervalOperations, sample.Operation), sample)
}

func recordTransactionHistogram(operation *operationHistogram, sample TransactionSample) {
	operation.outcomes.record(sample.Outcome)
	operation.cancellations.record(sample.Cancellation)
	operation.admission.record(sample.Admission)
	if sample.Outcome != OutcomeBeginFailed {
		operation.body.record(sample.Body)
		operation.hold.record(connectionHoldDuration(sample.Admission, sample.Total))
	}
	if sample.Outcome == OutcomeCommitted || sample.Outcome == OutcomeCommitFailed {
		operation.commit.record(sample.Commit)
	}
	operation.total.record(sample.Total)
}

func (r *Recorder) ObserveStatement(sample StatementSample) {
	if r == nil {
		return
	}
	descriptor, ok := sample.Operation.Descriptor()
	if !ok || descriptor.Kind != OperationKindStatement || sample.Role != descriptor.Role || sample.OperationName != "" && sample.OperationName != descriptor.Name {
		r.mu.Lock()
		r.rejectedSamples++
		r.intervalRejected++
		r.mu.Unlock()
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	recordStatementHistogram(statementHistogramFor(r.statements, sample.Operation), sample)
	recordStatementHistogram(statementHistogramFor(r.intervalStatements, sample.Operation), sample)
}

func recordStatementHistogram(statement *statementHistogram, sample StatementSample) {
	statement.outcomes.record(sample.Outcome)
	statement.cancellations.record(sample.Cancellation)
	statement.admission.record(sample.Admission)
	statement.hold.record(connectionHoldDuration(sample.Admission, sample.Total))
	statement.execution.record(sample.Execution)
	if sample.RowsObserved {
		statement.rowsLifetime.record(sample.RowsLifetime)
	}
	statement.total.record(sample.Total)
}

func (r *Recorder) ObserveRows(event RowsEvent) {
	if r == nil {
		return
	}
	descriptor, ok := event.Operation.Descriptor()
	if !ok || descriptor.Kind != OperationKindStatement || event.Role != descriptor.Role || event.OperationName != "" && event.OperationName != descriptor.Name {
		r.mu.Lock()
		r.rejectedSamples++
		r.intervalRejected++
		r.mu.Unlock()
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := &statementHistogramFor(r.statements, event.Operation).rows
	if event.Opened {
		rows.Opened++
		rows.Current++
		if rows.Current > rows.Peak {
			rows.Peak = rows.Current
		}
		return
	}
	rows.Closed++
	if rows.Current > 0 {
		rows.Current--
	} else {
		r.rejectedSamples++
	}
}

// Report snapshots all observed histograms in stable operation order.
func (r *Recorder) Report() Report {
	report := Report{GeneratedAt: time.Now().UTC()}
	if r == nil {
		return report
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	report.RejectedSamples = r.rejectedSamples
	for _, descriptor := range Operations() {
		switch descriptor.Kind {
		case OperationKindStatement:
			statement := r.statements[descriptor.Operation]
			if statement == nil {
				continue
			}
			report.Statements = append(report.Statements, StatementReport{
				Operation:     descriptor.Operation,
				Name:          descriptor.Name,
				Role:          descriptor.Role,
				Outcomes:      statement.outcomes,
				Cancellations: statement.cancellations,
				Admission:     statement.admission.report(),
				Hold:          statement.hold.report(),
				Execution:     statement.execution.report(),
				RowsLifetime:  statement.rowsLifetime.report(),
				Total:         statement.total.report(),
				Rows:          statement.rows,
			})
		default:
			operation := r.operations[descriptor.Operation]
			if operation == nil {
				continue
			}
			report.Operations = append(report.Operations, OperationReport{
				Operation:     descriptor.Operation,
				Name:          descriptor.Name,
				Role:          descriptor.Role,
				Outcomes:      operation.outcomes,
				Cancellations: operation.cancellations,
				Admission:     operation.admission.report(),
				Hold:          operation.hold.report(),
				Body:          operation.body.report(),
				Commit:        operation.commit.report(),
				Total:         operation.total.report(),
			})
		}
	}
	return report
}

// IntervalReport returns an exact, mergeable HDR interval and atomically
// rotates only the interval recorder. The cumulative Report remains intact.
// This lets an external controller separate warm-up, steady state, and drain
// without retaining individual SQL observations or introducing dynamic keys.
func (r *Recorder) IntervalReport() (Report, error) {
	report := Report{GeneratedAt: time.Now().UTC()}
	if r == nil {
		return report, nil
	}
	r.mu.Lock()
	operations := r.intervalOperations
	statements := r.intervalStatements
	report.RejectedSamples = r.intervalRejected
	r.intervalOperations = make(map[Operation]*operationHistogram)
	r.intervalStatements = make(map[Operation]*statementHistogram)
	r.intervalRejected = 0
	r.mu.Unlock()

	// Compression happens after the atomic rotation so transaction observers
	// never wait behind zlib while recording a completed catalog operation.
	for _, descriptor := range Operations() {
		switch descriptor.Kind {
		case OperationKindStatement:
			statement := statements[descriptor.Operation]
			if statement == nil {
				continue
			}
			item, err := encodedStatementReport(descriptor, statement)
			if err != nil {
				return Report{}, err
			}
			report.Statements = append(report.Statements, item)
		default:
			operation := operations[descriptor.Operation]
			if operation == nil {
				continue
			}
			item, err := encodedOperationReport(descriptor, operation)
			if err != nil {
				return Report{}, err
			}
			report.Operations = append(report.Operations, item)
		}
	}
	return report, nil
}

func operationHistogramFor(histograms map[Operation]*operationHistogram, operation Operation) *operationHistogram {
	histogram := histograms[operation]
	if histogram == nil {
		histogram = newOperationHistogram()
		histograms[operation] = histogram
	}
	return histogram
}

func statementHistogramFor(histograms map[Operation]*statementHistogram, operation Operation) *statementHistogram {
	statement := histograms[operation]
	if statement == nil {
		statement = newStatementHistogram()
		histograms[operation] = statement
	}
	return statement
}

func encodedOperationReport(descriptor OperationDescriptor, operation *operationHistogram) (OperationReport, error) {
	admission, err := operation.admission.encodedReport()
	if err != nil {
		return OperationReport{}, err
	}
	hold, err := operation.hold.encodedReport()
	if err != nil {
		return OperationReport{}, err
	}
	body, err := operation.body.encodedReport()
	if err != nil {
		return OperationReport{}, err
	}
	commit, err := operation.commit.encodedReport()
	if err != nil {
		return OperationReport{}, err
	}
	total, err := operation.total.encodedReport()
	if err != nil {
		return OperationReport{}, err
	}
	return OperationReport{
		Operation: descriptor.Operation, Name: descriptor.Name, Role: descriptor.Role,
		Outcomes: operation.outcomes, Cancellations: operation.cancellations,
		Admission: admission, Hold: hold, Body: body, Commit: commit, Total: total,
	}, nil
}

func encodedStatementReport(descriptor OperationDescriptor, statement *statementHistogram) (StatementReport, error) {
	admission, err := statement.admission.encodedReport()
	if err != nil {
		return StatementReport{}, err
	}
	hold, err := statement.hold.encodedReport()
	if err != nil {
		return StatementReport{}, err
	}
	execution, err := statement.execution.encodedReport()
	if err != nil {
		return StatementReport{}, err
	}
	rowsLifetime, err := statement.rowsLifetime.encodedReport()
	if err != nil {
		return StatementReport{}, err
	}
	total, err := statement.total.encodedReport()
	if err != nil {
		return StatementReport{}, err
	}
	return StatementReport{
		Operation: descriptor.Operation, Name: descriptor.Name, Role: descriptor.Role,
		Outcomes: statement.outcomes, Cancellations: statement.cancellations,
		Admission: admission, Hold: hold, Execution: execution, RowsLifetime: rowsLifetime, Total: total,
	}, nil
}

type operationHistogram struct {
	outcomes      OutcomeCounts
	cancellations CancellationCounts
	admission     latencyHistogram
	hold          latencyHistogram
	body          latencyHistogram
	commit        latencyHistogram
	total         latencyHistogram
}

type statementHistogram struct {
	outcomes      StatementOutcomeCounts
	cancellations CancellationCounts
	admission     latencyHistogram
	hold          latencyHistogram
	execution     latencyHistogram
	rowsLifetime  latencyHistogram
	total         latencyHistogram
	rows          RowsReport
}

func newStatementHistogram() *statementHistogram {
	return &statementHistogram{
		admission:    newLatencyHistogram(),
		hold:         newLatencyHistogram(),
		execution:    newLatencyHistogram(),
		rowsLifetime: newLatencyHistogram(),
		total:        newLatencyHistogram(),
	}
}

func newOperationHistogram() *operationHistogram {
	return &operationHistogram{
		admission: newLatencyHistogram(),
		hold:      newLatencyHistogram(),
		body:      newLatencyHistogram(),
		commit:    newLatencyHistogram(),
		total:     newLatencyHistogram(),
	}
}

// connectionHoldDuration is the exact time after database/sql admitted an
// operation until it released that physical connection. Admission queueing is
// deliberately excluded so writer monopolization and writer contention remain
// independently measurable.
func connectionHoldDuration(admission, total time.Duration) time.Duration {
	if total <= admission {
		return 0
	}
	return total - admission
}

type latencyHistogram struct {
	histogram *hdrhistogram.Histogram
	overflow  int64
}

func newLatencyHistogram() latencyHistogram {
	return latencyHistogram{histogram: hdrhistogram.New(
		histogramLowestMicroseconds,
		histogramHighestMicroseconds,
		histogramSignificantFigures,
	)}
}

func (h *latencyHistogram) record(duration time.Duration) {
	microseconds := duration.Microseconds()
	if duration > 0 && microseconds == 0 {
		microseconds = histogramLowestMicroseconds
	}
	if microseconds < 0 {
		microseconds = 0
	}
	if microseconds > histogramHighestMicroseconds {
		microseconds = histogramHighestMicroseconds
		h.overflow++
	}
	_ = h.histogram.RecordValue(microseconds)
}

func (h *latencyHistogram) report() HistogramReport {
	count := h.histogram.TotalCount()
	if count == 0 {
		return HistogramReport{}
	}
	return HistogramReport{
		Count:    count,
		P50:      durationFromMicroseconds(h.histogram.ValueAtQuantile(50)),
		P95:      durationFromMicroseconds(h.histogram.ValueAtQuantile(95)),
		P99:      durationFromMicroseconds(h.histogram.ValueAtQuantile(99)),
		P999:     durationFromMicroseconds(h.histogram.ValueAtQuantile(99.9)),
		Max:      durationFromMicroseconds(h.histogram.Max()),
		Overflow: h.overflow,
	}
}

func (h *latencyHistogram) encodedReport() (HistogramReport, error) {
	report := h.report()
	if report.Count == 0 {
		return report, nil
	}
	encoded, err := h.histogram.Encode(hdrhistogram.V2CompressedEncodingCookieBase)
	if err != nil {
		return HistogramReport{}, err
	}
	report.HDRV2 = string(encoded)
	return report, nil
}

func durationFromMicroseconds(value int64) time.Duration {
	return time.Duration(value) * time.Microsecond
}

// Report is the JSON-safe bounded summary used by runtime diagnostics and the
// pressure harness. Raw HDR logs are exported separately by the harness.
type Report struct {
	GeneratedAt     time.Time         `json:"generated_at"`
	RejectedSamples int64             `json:"rejected_samples"`
	Operations      []OperationReport `json:"operations"`
	Statements      []StatementReport `json:"statements"`
}

func (r Report) Operation(operation Operation) (OperationReport, bool) {
	for _, candidate := range r.Operations {
		if candidate.Operation == operation {
			return candidate, true
		}
	}
	return OperationReport{}, false
}

func (r Report) Statement(operation Operation) (StatementReport, bool) {
	for _, candidate := range r.Statements {
		if candidate.Operation == operation {
			return candidate, true
		}
	}
	return StatementReport{}, false
}

type OperationReport struct {
	Operation     Operation          `json:"-"`
	Name          string             `json:"name"`
	Role          Role               `json:"role"`
	Outcomes      OutcomeCounts      `json:"outcomes"`
	Cancellations CancellationCounts `json:"cancellations"`
	Admission     HistogramReport    `json:"admission"`
	Hold          HistogramReport    `json:"hold"`
	Body          HistogramReport    `json:"body"`
	Commit        HistogramReport    `json:"commit"`
	Total         HistogramReport    `json:"total"`
}

type StatementReport struct {
	Operation     Operation              `json:"-"`
	Name          string                 `json:"name"`
	Role          Role                   `json:"role"`
	Outcomes      StatementOutcomeCounts `json:"outcomes"`
	Cancellations CancellationCounts     `json:"cancellations"`
	Admission     HistogramReport        `json:"admission"`
	Hold          HistogramReport        `json:"hold"`
	Execution     HistogramReport        `json:"execution"`
	RowsLifetime  HistogramReport        `json:"rows_lifetime"`
	Total         HistogramReport        `json:"total"`
	Rows          RowsReport             `json:"rows"`
}

type RowsReport struct {
	Current int64 `json:"current"`
	Peak    int64 `json:"peak"`
	Opened  int64 `json:"opened"`
	Closed  int64 `json:"closed"`
}

type HistogramReport struct {
	Count    int64         `json:"count"`
	P50      time.Duration `json:"p50_ns"`
	P95      time.Duration `json:"p95_ns"`
	P99      time.Duration `json:"p99_ns"`
	P999     time.Duration `json:"p999_ns"`
	Max      time.Duration `json:"max_ns"`
	Overflow int64         `json:"overflow"`
	HDRV2    string        `json:"hdr_v2,omitempty"`
}

type OutcomeCounts struct {
	Committed      int64 `json:"committed"`
	BeginFailed    int64 `json:"begin_failed"`
	RolledBack     int64 `json:"rolled_back"`
	RollbackFailed int64 `json:"rollback_failed"`
	CommitFailed   int64 `json:"commit_failed"`
	Panicked       int64 `json:"panicked"`
	Unknown        int64 `json:"unknown"`
}

type StatementOutcomeCounts struct {
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
	Unknown   int64 `json:"unknown"`
}

func (c *StatementOutcomeCounts) record(outcome StatementOutcome) {
	switch outcome {
	case StatementOutcomeSucceeded:
		c.Succeeded++
	case StatementOutcomeFailed:
		c.Failed++
	default:
		c.Unknown++
	}
}

func (c *OutcomeCounts) record(outcome Outcome) {
	switch outcome {
	case OutcomeCommitted:
		c.Committed++
	case OutcomeBeginFailed:
		c.BeginFailed++
	case OutcomeRolledBack:
		c.RolledBack++
	case OutcomeRollbackFailed:
		c.RollbackFailed++
	case OutcomeCommitFailed:
		c.CommitFailed++
	case OutcomePanicked:
		c.Panicked++
	default:
		c.Unknown++
	}
}

type CancellationCounts struct {
	None             int64 `json:"none"`
	Canceled         int64 `json:"canceled"`
	DeadlineExceeded int64 `json:"deadline_exceeded"`
}

func (c *CancellationCounts) record(cancellation Cancellation) {
	switch cancellation {
	case CancellationCanceled:
		c.Canceled++
	case CancellationDeadlineExceeded:
		c.DeadlineExceeded++
	default:
		c.None++
	}
}
