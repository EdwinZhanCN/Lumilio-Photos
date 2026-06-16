-- Revert to the (probability-scale) bar from migration 034.
UPDATE classifier_definitions
SET threshold = 0.2,
    updated_at = NOW();
