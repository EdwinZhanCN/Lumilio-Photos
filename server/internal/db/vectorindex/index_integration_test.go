//go:build cgo

package vectorindex

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"testing"

	"server/internal/db/catalogtx"
	"server/internal/db/vec1ext"

	_ "github.com/mattn/go-sqlite3"
)

func TestTrainANNInstallsQueryableModel(t *testing.T) {
	vec1ext.Auto()
	ctx := context.Background()
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = database.Close() })

	if _, err := database.ExecContext(ctx, `
		CREATE TABLE search_embeddings (
			id INTEGER PRIMARY KEY,
			vector BLOB NOT NULL
		) STRICT;
		CREATE TABLE semantic_vector_index_state (
			id INTEGER PRIMARY KEY,
			mode TEXT NOT NULL,
			row_count INTEGER NOT NULL,
			trained_row_count INTEGER NOT NULL,
			rebuild_pending INTEGER NOT NULL,
			config TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		) STRICT;
		INSERT INTO semantic_vector_index_state
		VALUES (1, 'flat', 512, 0, 1, '{"index":"flat","distance":"l2"}', 0);
		CREATE VIRTUAL TABLE search_embeddings_vec USING vec1(embedding);
		INSERT INTO search_embeddings_vec(cmd, arg)
		VALUES ('rebuild', '{"index":"flat","distance":"l2"}');
	`); err != nil {
		t.Fatal(err)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for rowID := 1; rowID <= 512; rowID++ {
		vector := trainingVector(rowID, 16)
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO search_embeddings(id, vector) VALUES (?, ?)",
			rowID,
			vector,
		); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO search_embeddings_vec(rowid, embedding) VALUES (?, ?)",
			rowID,
			vector,
		); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := trainANN(ctx, catalogtx.NewWriter(database, nil), database, 512); err != nil {
		t.Fatal(err)
	}
	if mode, err := CurrentMode(ctx, database); err != nil || mode != ModeANN {
		t.Fatalf("Vec1 mode = %q, error = %v", mode, err)
	}

	var rowID int64
	if err := database.QueryRowContext(ctx, `
		SELECT rowid
		FROM search_embeddings_vec(?, '{"k":5,"nprobe":1.0}')
		LIMIT 1
	`, trainingVector(10, 16)).Scan(&rowID); err != nil {
		t.Fatal(err)
	}
	if rowID <= 0 {
		t.Fatalf("ANN rowid = %d", rowID)
	}

	recallHits := 0
	const recallQueries = 20
	for seed := 600; seed < 600+recallQueries; seed++ {
		query := trainingVector(seed, 16)
		var exactRowID int64
		if err := database.QueryRowContext(ctx, `
			SELECT id
			FROM search_embeddings
			ORDER BY vec1_l2_distance(vector, ?), id
			LIMIT 1
		`, query).Scan(&exactRowID); err != nil {
			t.Fatal(err)
		}
		if annContainsRow(ctx, t, database, query, exactRowID) {
			recallHits++
		}
	}
	if recallHits < 16 {
		t.Fatalf("ANN recall@16 = %d/%d, want at least 16/%d", recallHits, recallQueries, recallQueries)
	}
}

func annContainsRow(ctx context.Context, t *testing.T, database *sql.DB, query []byte, target int64) bool {
	t.Helper()
	rows, err := database.QueryContext(ctx, `
		SELECT rowid
		FROM search_embeddings_vec(?, '{"k":16,"nprobe":0.15}')
	`, query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var rowID int64
		if err := rows.Scan(&rowID); err != nil {
			t.Fatal(err)
		}
		if rowID == target {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}

func trainingVector(seed, dimensions int) []byte {
	values := make([]float64, dimensions)
	var norm float64
	for index := range values {
		value := math.Sin(float64(seed*(index+1))) + math.Cos(float64(seed+index*17))
		values[index] = value
		norm += value * value
	}
	norm = math.Sqrt(norm)
	blob := make([]byte, dimensions*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(
			blob[index*4:],
			math.Float32bits(float32(value/norm)),
		)
	}
	return blob
}
