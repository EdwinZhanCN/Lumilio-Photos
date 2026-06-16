-- Revert to the contrastive-era preset threshold.
UPDATE classifier_definitions
SET threshold = 0.05,
    updated_at = NOW();
