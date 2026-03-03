UPDATE plugins
SET panel = 'plugins',
    updated_at = datetime('now')
WHERE panel IN ('frames', 'develop');
