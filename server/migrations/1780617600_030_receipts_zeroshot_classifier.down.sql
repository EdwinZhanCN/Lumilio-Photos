DELETE FROM classifier_definitions
WHERE slug = 'receipts';

DROP INDEX IF EXISTS idx_asset_tags_tag_source_asset;

UPDATE classifier_definitions
SET positive_prompts = ARRAY[
        'a scanned document',
        'a photo of a page of text',
        'a document or paperwork',
        'a receipt or invoice',
        'a page from a book or contract'
    ],
    negative_prompts = ARRAY[
        'a natural scene photograph',
        'a photo of people',
        'an outdoor landscape'
    ],
    updated_at = NOW(),
    positive_prototype = NULL,
    negative_prototype = NULL,
    prototype_model = NULL,
    prototype_dimensions = NULL,
    prototype_built_at = NULL
WHERE slug = 'documents';
