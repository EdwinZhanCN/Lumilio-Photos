UPDATE classifier_definitions
SET positive_prompts = ARRAY[
        'a scanned document',
        'a photo of a page of text',
        'a document or paperwork',
        'a page from a book or contract',
        'an official form or letter'
    ],
    negative_prompts = ARRAY[
        'a natural scene photograph',
        'a photo of people',
        'an outdoor landscape',
        'a receipt or invoice'
    ],
    updated_at = NOW(),
    positive_prototype = NULL,
    negative_prototype = NULL,
    prototype_model = NULL,
    prototype_dimensions = NULL,
    prototype_built_at = NULL
WHERE slug = 'documents';

CREATE INDEX IF NOT EXISTS idx_asset_tags_tag_source_asset ON asset_tags(tag_id, source, asset_id);

INSERT INTO classifier_definitions (
    slug,
    display_name,
    tag_name,
    category,
    positive_prompts,
    negative_prompts,
    threshold
) VALUES (
    'receipts',
    'Receipts',
    'receipt',
    'smart_album',
    ARRAY[
        'a receipt',
        'a store receipt',
        'a restaurant receipt',
        'a photo of an invoice',
        'a bill or purchase receipt'
    ],
    ARRAY[
        'a natural scene photograph',
        'a photo of people',
        'an outdoor landscape',
        'a page from a book or contract'
    ],
    0.05
)
ON CONFLICT (slug) DO UPDATE
SET display_name = EXCLUDED.display_name,
    tag_name = EXCLUDED.tag_name,
    category = EXCLUDED.category,
    positive_prompts = EXCLUDED.positive_prompts,
    negative_prompts = EXCLUDED.negative_prompts,
    threshold = EXCLUDED.threshold,
    updated_at = NOW(),
    positive_prototype = NULL,
    negative_prototype = NULL,
    prototype_model = NULL,
    prototype_dimensions = NULL,
    prototype_built_at = NULL;
