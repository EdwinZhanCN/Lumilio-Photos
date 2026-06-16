-- Zero-shot classifier scoring moved from a contrastive cosine margin to
-- SigLIP's calibrated match probability p = sigmoid(exp(logit_scale)*cos +
-- logit_bias). Thresholds are now probabilities in [0,1]; re-seed the presets
-- to the balanced bar (~0.2) used by semantic set search. Negative prototypes
-- are obsolete (the logit bias already encodes the background), so clear them.
UPDATE classifier_definitions
SET threshold = 0.2,
    negative_prototype = NULL,
    updated_at = NOW();
