-- Runtime toggle for the zero-shot classification pipeline.
ALTER TABLE settings
ADD COLUMN IF NOT EXISTS ml_zeroshot_classify_enabled BOOLEAN NOT NULL DEFAULT false;

-- Allow the dedicated tag source written by the zero-shot classifier.
ALTER TABLE asset_tags
DROP CONSTRAINT IF EXISTS asset_tags_source_check;

ALTER TABLE asset_tags
ADD CONSTRAINT asset_tags_source_check
CHECK (source IN (
    'system',
    'user',
    'ai',
    'bioclip_classify',
    'zeroshot'
));

-- Classifier definitions: a "smart album" recipe expressed as a prompt ensemble.
-- Positive/negative prototypes are cached unit vectors (mean-pooled, L2-normalized
-- text embeddings) so the async worker never re-runs the text model per asset.
CREATE TABLE classifier_definitions (
    id SERIAL PRIMARY KEY,
    slug VARCHAR(64) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    tag_name VARCHAR(50) NOT NULL,
    category VARCHAR(50) NOT NULL DEFAULT 'smart_album',
    positive_prompts TEXT[] NOT NULL,
    negative_prompts TEXT[] NOT NULL DEFAULT '{}',
    threshold REAL NOT NULL DEFAULT 0.0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    positive_prototype VECTOR,
    negative_prototype VECTOR,
    prototype_model VARCHAR(100),
    prototype_dimensions INTEGER,
    prototype_built_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_classifier_definitions_enabled ON classifier_definitions(enabled);

-- Seed preset categories. Prototypes are left NULL and built lazily on first run
-- (against whatever semantic text model the deployment uses). Thresholds are conservative
-- defaults on the contrastive score; tune later via the preview endpoint.
INSERT INTO classifier_definitions (slug, display_name, tag_name, category, positive_prompts, negative_prompts, threshold) VALUES
(
    'documents',
    'Documents',
    'document',
    'smart_album',
    ARRAY[
        'a scanned document',
        'a photo of a page of text',
        'a document or paperwork',
        'a receipt or invoice',
        'a page from a book or contract'
    ],
    ARRAY[
        'a natural scene photograph',
        'a photo of people',
        'an outdoor landscape'
    ],
    0.05
),
(
    'screenshots',
    'Screenshots',
    'screenshot',
    'smart_album',
    ARRAY[
        'a screenshot of a computer screen',
        'a screenshot of a phone app',
        'a screenshot of a user interface',
        'a screen capture of a website'
    ],
    ARRAY[
        'a natural scene photograph',
        'a printed photograph'
    ],
    0.05
),
(
    'illustration',
    'Illustration',
    'illustration',
    'smart_album',
    ARRAY[
        'a digital illustration',
        'a drawing or artwork',
        'a cartoon or anime image',
        'a painting',
        'computer generated art'
    ],
    ARRAY[
        'a real photograph',
        'a photo taken with a camera'
    ],
    0.05
);
