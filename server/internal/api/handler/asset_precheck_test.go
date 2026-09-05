package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"server/internal/api/dto"
	"server/internal/db/repo"
	"server/internal/utils/hash"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPrecheckFullAndQuickMembershipSets(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer database.Close()
	database.SetMaxOpenConns(1)
	for _, stmt := range []string{
		`CREATE TABLE repositories(repo_id TEXT, name TEXT, path TEXT, config TEXT, reachability TEXT, activity TEXT, pause_reason TEXT,last_sync INTEGER,created_at INTEGER,updated_at INTEGER,default_owner_id INTEGER,role TEXT,root_id TEXT)`,
		`CREATE TABLE assets(asset_id TEXT,original_filename TEXT,content_id TEXT)`,
		`CREATE TABLE content_objects(content_id TEXT,full_hash TEXT,file_size INTEGER)`,
		`CREATE TABLE active_asset_occurrences(asset_id TEXT,repository_id TEXT,quick_fingerprint TEXT,file_size INTEGER)`,
	} {
		_, err = database.Exec(stmt)
		require.NoError(t, err)
	}
	rid, aid := uuid.NewString(), uuid.NewString()
	_, err = database.Exec(`INSERT INTO repositories VALUES (?, 'test','/tmp','{}','active','idle','',0,0,0,1,'regular',?)`, rid, uuid.NewString())
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO assets VALUES (?, 'test.jpg', 'content')`, aid)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO content_objects VALUES ('content','full',42)`)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO active_asset_occurrences VALUES (?,?,'quick',42)`, aid, rid)
	require.NoError(t, err)
	h := &AssetHandler{queries: repo.New(database)}
	version := hash.QuickFingerprintVersion
	full := dto.UploadPrecheckFileDTO{Hash: "full", Size: 42}
	quick := dto.UploadPrecheckFileDTO{Hash: "quick", Size: 42, IsQuick: true, FingerprintVersion: &version}
	for _, tc := range []struct {
		name       string
		files      []dto.UploadPrecheckFileDTO
		candidates int
	}{
		{"full only", []dto.UploadPrecheckFileDTO{full}, 1},
		{"quick only", []dto.UploadPrecheckFileDTO{quick}, 1},
		{"mixed", []dto.UploadPrecheckFileDTO{full, quick}, 2},
		{"unknown quick version", []dto.UploadPrecheckFileDTO{{Hash: "quick", Size: 42, IsQuick: true}}, 0},
		{"size mismatch", []dto.UploadPrecheckFileDTO{{Hash: "full", Size: 43}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := postJSON(t, "/api/v1/assets/precheck", dto.UploadPrecheckRequestDTO{RepositoryID: rid, Files: tc.files}, h.PrecheckUpload)
			require.Equal(t, http.StatusOK, r.Code, r.Body.String())
			var result dto.UploadPrecheckResponseDTO
			require.NoError(t, json.Unmarshal(r.Body.Bytes(), &result))
			require.Len(t, result.Results, len(tc.files))
			require.Equal(t, tc.candidates, result.DuplicateCount)
			for _, v := range result.Results {
				require.False(t, v.Duplicate)
				if v.Candidate {
					require.Equal(t, aid, *v.AssetID)
				}
			}
		})
	}
}
