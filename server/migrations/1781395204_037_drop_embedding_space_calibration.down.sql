ALTER TABLE embedding_spaces
    ADD COLUMN logit_scale DOUBLE PRECISION,
    ADD COLUMN logit_bias DOUBLE PRECISION;
