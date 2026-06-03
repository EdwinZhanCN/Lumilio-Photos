-- Revert neutral semantic naming back to the model-specific "clip" identifiers.

UPDATE embedding_spaces SET embedding_type = 'clip' WHERE embedding_type = 'semantic';
UPDATE embeddings SET embedding_type = 'clip' WHERE embedding_type = 'semantic';

ALTER TABLE settings RENAME COLUMN ml_semantic_enabled TO ml_clip_enabled;
