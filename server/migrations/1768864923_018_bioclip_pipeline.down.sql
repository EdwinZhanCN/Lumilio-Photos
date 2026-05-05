UPDATE asset_tags
SET source = 'ai'
WHERE source = 'bioclip_classify';

ALTER TABLE asset_tags
DROP CONSTRAINT IF EXISTS asset_tags_source_check;

ALTER TABLE asset_tags
ADD CONSTRAINT asset_tags_source_check
CHECK (source IN ('system', 'user', 'ai'));

ALTER TABLE settings
DROP COLUMN IF EXISTS ml_bioclip_enabled;
