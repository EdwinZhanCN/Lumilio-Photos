CREATE INDEX IF NOT EXISTS idx_assets_focal_length_active
    ON assets (json_extract(specific_metadata, '$.focal_length'))
    WHERE is_deleted = 0
      AND json_type(specific_metadata, '$.focal_length') IN ('integer', 'real');
