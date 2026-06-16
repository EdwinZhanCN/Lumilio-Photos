-- Revert to the cosine-floor threshold from migration 035.
UPDATE classifier_definitions
SET threshold = 0.105,
    updated_at = NOW();
