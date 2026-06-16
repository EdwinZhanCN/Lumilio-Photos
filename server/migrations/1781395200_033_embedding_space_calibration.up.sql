-- SigLIP-style absolute calibration for an embedding space. Raw learned
-- scalars (log-space scale + bias) reported by the ML server on embedding
-- responses, so search/classify can map cosine similarity to a calibrated
-- match probability: sigmoid(exp(logit_scale)*cos + logit_bias).
-- Nullable: non-calibrated models (or rows created before the first calibrated
-- embedding) leave these NULL and fall back to uncalibrated behavior.
ALTER TABLE embedding_spaces
    ADD COLUMN logit_scale DOUBLE PRECISION,
    ADD COLUMN logit_bias DOUBLE PRECISION;
