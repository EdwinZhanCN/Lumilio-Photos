DROP TABLE IF EXISTS classifier_definitions;

UPDATE asset_tags
SET source = 'ai'
WHERE source = 'zeroshot';

ALTER TABLE asset_tags
DROP CONSTRAINT IF EXISTS asset_tags_source_check;

ALTER TABLE asset_tags
ADD CONSTRAINT asset_tags_source_check
CHECK (source IN (
    'system',
    'user',
    'ai',
    'bioclip_classify'
));

ALTER TABLE settings
DROP COLUMN IF EXISTS ml_zeroshot_classify_enabled;
