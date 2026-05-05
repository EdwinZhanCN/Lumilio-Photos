ALTER TABLE settings
ADD COLUMN IF NOT EXISTS ml_bioclip_enabled BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE asset_tags
DROP CONSTRAINT IF EXISTS asset_tags_source_check;

UPDATE asset_tags
SET source = 'ai'
WHERE source IN ('clip_classify', 'clip_scene_classify');

ALTER TABLE asset_tags
ADD CONSTRAINT asset_tags_source_check
CHECK (source IN (
    'system',
    'user',
    'ai',
    'bioclip_classify'
));
