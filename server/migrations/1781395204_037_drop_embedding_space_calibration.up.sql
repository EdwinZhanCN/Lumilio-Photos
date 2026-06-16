-- SigLIP absolute calibration was abandoned: zero-shot probabilities are
-- intrinsically tiny, so search uses a cosine floor and classify uses a
-- contrastive margin — neither needs per-space logit_scale/logit_bias. Drop the
-- now-unused columns added in migration 033.
ALTER TABLE embedding_spaces
    DROP COLUMN IF EXISTS logit_scale,
    DROP COLUMN IF EXISTS logit_bias;
