-- Neutral semantic naming: decouple the generic image/text embedding slot from a
-- specific model name. The runtime toggle column and the persisted embedding_type
-- value are renamed from the model-specific "clip" to the model-agnostic
-- "semantic". The Lumen SDK model identity (e.g. SigLIP) is unaffected; this only
-- touches the system-level semantics that the application and frontend rely on.

ALTER TABLE settings RENAME COLUMN ml_clip_enabled TO ml_semantic_enabled;

UPDATE embeddings SET embedding_type = 'semantic' WHERE embedding_type = 'clip';
UPDATE embedding_spaces SET embedding_type = 'semantic' WHERE embedding_type = 'clip';
