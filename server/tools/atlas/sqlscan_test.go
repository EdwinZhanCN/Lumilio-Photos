package main

import (
	"sort"
	"strings"
	"testing"
)

func accesses(sql string) string {
	var out []string
	for _, statement := range parseSQL(sql) {
		prefix := ""
		if statement.Trigger != "" {
			prefix = "trigger:" + statement.Trigger + " "
		}
		for _, access := range statement.Accesses {
			out = append(out, prefix+access.Op+" "+access.Table)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func TestSQLScannerFindsEveryTable(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{`DELETE FROM assets WHERE id = ?`, "delete assets"},
		{`delete from "Assets" where id in (select asset_id from entries)`, "delete assets, read entries"},
		{`INSERT OR REPLACE INTO thumbnails (id) VALUES (?)`, "insert thumbnails"},
		{`REPLACE INTO thumbnails (id) SELECT id FROM assets`, "insert thumbnails, read assets"},
		{`UPDATE OR IGNORE assets SET x = 1 FROM media m WHERE m.id = assets.id`, "read media, update assets"},
		{`INSERT INTO tags (name) VALUES (?) ON CONFLICT (name) DO UPDATE SET name = excluded.name`, "insert tags"},
		{`WITH RECURSIVE gone(id) AS (SELECT id FROM assets WHERE dead) DELETE FROM assets WHERE id IN gone`, "delete assets, read assets"},
		{`WITH a AS (SELECT 1), b AS NOT MATERIALIZED (SELECT * FROM river_job) SELECT * FROM a JOIN b`, "read river_job"},
		{`SELECT * FROM assets a, main.entries e LEFT JOIN media m ON m.id = a.id`, "read assets, read entries, read media"},
		{`SELECT value FROM json_each(?)`, ""},
		{`SELECT 'DELETE FROM assets' -- DELETE FROM assets
		  /* UPDATE assets */ FROM notes`, "read notes"},
		{"SELECT 1; DELETE FROM a; UPDATE b SET c = 1", "delete a, update b"},
		{`CREATE TABLE assets (id INTEGER REFERENCES media(id) ON DELETE CASCADE)`, ""},
		{`CREATE TRIGGER IF NOT EXISTS purge AFTER UPDATE ON entries BEGIN
			DELETE FROM assets WHERE id = NEW.id;
			UPDATE media SET n = n - 1;
		  END;
		  DELETE FROM later`, "delete later, trigger:purge delete assets, trigger:purge update media"},
	} {
		if got := accesses(tc.sql); got != tc.want {
			t.Errorf("%s\n got: %s\nwant: %s", tc.sql, got, tc.want)
		}
	}
}

func TestSQLScannerRecognizesCountQueries(t *testing.T) {
	for sql, want := range map[string]bool{
		`SELECT count(*) FROM assets`:                                true,
		`SELECT COUNT(DISTINCT id) AS total FROM assets WHERE x = ?`: true,
		`SELECT count(*) total FROM assets`:                          true,
		`SELECT id, count(*) FROM assets GROUP BY id`:                false,
		`SELECT id FROM assets WHERE (SELECT count(*) FROM x) > 1`:   false,
		`SELECT count(*) + 1 FROM assets`:                            false,
	} {
		statements := parseSQL(sql)
		if len(statements) != 1 || statements[0].Count != want {
			t.Errorf("%s: count = %v, want %v", sql, len(statements) == 1 && statements[0].Count, want)
		}
	}
}
