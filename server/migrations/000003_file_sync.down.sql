-- File synchronization tracking migration (down)
-- Drops tables created for file tracking and sync operations

DROP INDEX IF EXISTS idx_sync_operations_status;
DROP INDEX IF EXISTS idx_sync_operations_start_time;
DROP INDEX IF EXISTS idx_sync_operations_repo_id;

DROP INDEX IF EXISTS idx_file_records_hash;
DROP INDEX IF EXISTS idx_file_records_mod_time;
DROP INDEX IF EXISTS idx_file_records_scan_gen;
DROP INDEX IF EXISTS idx_file_records_repo_path;
DROP INDEX IF EXISTS idx_file_records_repo_id;

DROP TABLE IF EXISTS sync_operations;
DROP TABLE IF EXISTS file_records;
