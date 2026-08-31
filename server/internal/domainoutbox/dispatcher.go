// Package domainoutbox delivers catalog domain commands to a disposable
// control plane without consulting that control plane for product truth.
package domainoutbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/pipeline"
)

type Entry struct {
	ID             string
	Kind           string
	SubjectKey     string
	DesiredVersion uint64
	Envelope       pipeline.Envelope
}

// Adapter is implemented only by the River boundary. InsertMany must be
// at-least-once safe and map the closed command kinds to macro jobs.
type Adapter interface {
	InsertMany(context.Context, []Entry) error
}

type Dispatcher struct {
	reader       *catalogtx.Reader
	writer       *catalogtx.Writer
	adapter      Adapter
	batchSize    int
	pollInterval time.Duration
}

func NewDispatcher(reader *catalogtx.Reader, writer *catalogtx.Writer, adapter Adapter, batchSize int, pollInterval time.Duration) (*Dispatcher, error) {
	if reader == nil || reader.Pool() == nil || writer == nil || writer.Pool() == nil || adapter == nil || batchSize < 1 || pollInterval <= 0 {
		return nil, errors.New("domain outbox dispatcher requires catalog capabilities, adapter, and positive bounds")
	}
	return &Dispatcher{reader: reader, writer: writer, adapter: adapter, batchSize: batchSize, pollInterval: pollInterval}, nil
}

func (d *Dispatcher) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("domain outbox dispatcher context is nil")
	}
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()
	for {
		if _, err := d.DeliverOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				return ctx.Err()
			}
			// Delivery failures are durable in the outbox rows. Keep the
			// dispatcher alive and let the next bounded poll retry them; a
			// transient QueueDB outage must not stop catalog recovery.
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) DeliverOnce(ctx context.Context) (int, error) {
	if ctx == nil {
		return 0, errors.New("domain outbox delivery context is nil")
	}
	rows, err := d.reader.Pool().QueryContext(ctx, `
		SELECT outbox_id, command_kind, subject_key, desired_version, envelope
		FROM domain_outbox
		WHERE delivered_at IS NULL AND available_at <= ?
		ORDER BY available_at, created_at, outbox_id
		LIMIT ?`, time.Now().UTC().UnixMicro(), d.batchSize)
	if err != nil {
		return 0, fmt.Errorf("read domain outbox: %w", err)
	}
	defer func() { _ = rows.Close() }()
	entries := make([]Entry, 0, d.batchSize)
	for rows.Next() {
		var entry Entry
		var encoded string
		if err := rows.Scan(&entry.ID, &entry.Kind, &entry.SubjectKey, &entry.DesiredVersion, &encoded); err != nil {
			return 0, err
		}
		if err := json.Unmarshal([]byte(encoded), &entry.Envelope); err != nil {
			return 0, fmt.Errorf("decode outbox %s: %w", entry.ID, err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close domain outbox rows: %w", err)
	}
	if len(entries) == 0 {
		return 0, nil
	}
	if err := d.adapter.InsertMany(ctx, entries); err != nil {
		d.recordFailure(ctx, entries, err)
		return 0, fmt.Errorf("bulk insert macro jobs: %w", err)
	}
	now := time.Now().UTC().UnixMicro()
	err = d.writer.Transact(ctx, catalogtx.OperationDomainOutboxDeliver, nil, func(tx *sql.Tx) error {
		for _, entry := range entries {
			if _, err := tx.ExecContext(ctx, `UPDATE domain_outbox SET delivered_at = ?, delivery_attempts = delivery_attempts + 1, last_error = NULL, updated_at = ? WHERE outbox_id = ? AND delivered_at IS NULL`, now, now, entry.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("acknowledge domain outbox: %w", err)
	}
	return len(entries), nil
}

func (d *Dispatcher) recordFailure(ctx context.Context, entries []Entry, cause error) {
	now := time.Now().UTC().UnixMicro()
	available := time.Now().UTC().Add(d.pollInterval).UnixMicro()
	_ = d.writer.Transact(ctx, catalogtx.OperationDomainOutboxDeliver, nil, func(tx *sql.Tx) error {
		for _, entry := range entries {
			if _, err := tx.ExecContext(ctx, `UPDATE domain_outbox SET delivery_attempts = delivery_attempts + 1, last_error = ?, available_at = ?, updated_at = ? WHERE outbox_id = ? AND delivered_at IS NULL`, cause.Error(), available, now, entry.ID); err != nil {
				return err
			}
		}
		return nil
	})
}
