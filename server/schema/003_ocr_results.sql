-- OCR Results Table
-- Store text recognition results for images

-- Create OCR results main table
CREATE TABLE ocr_results (
    asset_id UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    model_id VARCHAR(100) NOT NULL,           -- OCR model identifier
    total_count INTEGER NOT NULL DEFAULT 0,  -- Total number of detected text regions
    processing_time_ms INTEGER,              -- Processing time in milliseconds
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (asset_id)
);

-- Create OCR text items table (for each detected text region)
CREATE TABLE ocr_text_items (
    id SERIAL PRIMARY KEY,
    asset_id UUID NOT NULL REFERENCES ocr_results(asset_id) ON DELETE CASCADE,
    text_content TEXT NOT NULL,              -- Recognized text content
    confidence REAL NOT NULL,                -- Recognition confidence (0.0-1.0)
    bounding_box JSONB NOT NULL,             -- Polygon coordinates: [[x1,y1], [x2,y2], [x3,y3], [x4,y4]]
    text_length INTEGER NOT NULL,            -- Text length
    area_pixels REAL,                        -- Text region area in pixels
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Index optimization
-- Main table indexes
CREATE INDEX ocr_results_asset_id_idx ON ocr_results(asset_id);
CREATE INDEX ocr_results_model_id_idx ON ocr_results(model_id);
CREATE INDEX ocr_results_created_at_idx ON ocr_results(created_at);

-- Text items table indexes
CREATE INDEX ocr_text_items_asset_id_idx ON ocr_text_items(asset_id);
CREATE INDEX ocr_text_items_confidence_idx ON ocr_text_items(confidence);
CREATE INDEX ocr_text_items_text_length_idx ON ocr_text_items(text_length);

-- Full-text search index (supports Chinese and English)
CREATE INDEX ocr_text_items_fulltext_idx ON ocr_text_items
USING GIN (to_tsvector('simple', text_content));

-- GIS index (if spatial queries are needed, install PostGIS extension)
-- CREATE EXTENSION IF NOT EXISTS postgis;
-- CREATE INDEX ocr_text_items_bbox_idx ON ocr_text_items
-- USING GIST (ST_EnvelopeFromJSON(bounding_box));

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_ocr_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE ocr_results
    SET updated_at = CURRENT_TIMESTAMP
    WHERE asset_id = NEW.asset_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ocr_text_items_update_trigger
    AFTER INSERT OR UPDATE OR DELETE ON ocr_text_items
    FOR EACH ROW
    EXECUTE FUNCTION update_ocr_updated_at();

-- Add constraints
ALTER TABLE ocr_results ADD CONSTRAINT chk_total_count_nonnegative
CHECK (total_count >= 0);

ALTER TABLE ocr_text_items ADD CONSTRAINT chk_confidence_range
CHECK (confidence >= 0.0 AND confidence <= 1.0);

-- Comments
COMMENT ON TABLE ocr_results IS 'OCR recognition results main table, storing OCR processing summary for each image';
COMMENT ON TABLE ocr_text_items IS 'OCR recognized text items table, storing detailed information for each text region';
COMMENT ON COLUMN ocr_results.model_id IS 'OCR model identifier, such as "paddleocr-v2", "easyocr-v1", etc.';
COMMENT ON COLUMN ocr_results.total_count IS 'Total number of detected text regions';
COMMENT ON COLUMN ocr_results.processing_time_ms IS 'OCR processing time in milliseconds';
COMMENT ON COLUMN ocr_text_items.text_content IS 'Recognized text content, supports multiple languages';
COMMENT ON COLUMN ocr_text_items.confidence IS 'Recognition confidence score, between 0.0-1.0';
COMMENT ON COLUMN ocr_text_items.bounding_box IS 'Polygon coordinates, format: [[x1,y1], [x2,y2], [x3,y3], [x4,y4]]';
COMMENT ON COLUMN ocr_text_items.area_pixels IS 'Approximate area of text region, can be used to filter larger text';