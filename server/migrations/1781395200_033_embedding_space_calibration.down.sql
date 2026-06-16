ALTER TABLE embedding_spaces
    DROP COLUMN IF EXISTS logit_scale,
    DROP COLUMN IF EXISTS logit_bias;
