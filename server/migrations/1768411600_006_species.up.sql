-- Species predictions table for image classification results
CREATE TABLE IF NOT EXISTS species_predictions (
    prediction_id SERIAL PRIMARY KEY,
    asset_id UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    label VARCHAR(255) NOT NULL,
    score REAL NOT NULL CHECK (score >= 0 AND score <= 1),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (asset_id, label)
);

-- Indexes for species predictions
CREATE INDEX IF NOT EXISTS idx_species_predictions_asset_id ON species_predictions(asset_id);
CREATE INDEX IF NOT EXISTS idx_species_predictions_label ON species_predictions(label);
CREATE INDEX IF NOT EXISTS idx_species_predictions_score ON species_predictions(score DESC);
CREATE INDEX IF NOT EXISTS idx_species_predictions_label_score ON species_predictions(label, score DESC);

-- Composite index for efficient label search with high scores
CREATE INDEX IF NOT EXISTS idx_species_predictions_label_asset_score
    ON species_predictions(label, asset_id, score DESC)
    WHERE score >= 0.5;
