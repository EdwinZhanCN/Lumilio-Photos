-- Down migration for 000003_repositories.up.sql
-- Drops the repositories table and its indexes in a safe, dependency-aware order.
-- Uses IF EXISTS guards to make the migration idempotent.

-- Drop indexes first (though they would be dropped automatically with the table)
DROP INDEX IF EXISTS idx_repositories_path;
DROP INDEX IF EXISTS idx_repositories_status;

-- Drop the repositories table
DROP TABLE IF EXISTS repositories;
