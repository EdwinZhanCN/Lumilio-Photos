-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP INDEX IF EXISTS idx_species_predictions_label_trgm;
DROP TABLE IF EXISTS species_predictions CASCADE;
