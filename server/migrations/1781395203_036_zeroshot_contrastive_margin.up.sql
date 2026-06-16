-- Zero-shot classification reverts to the canonical contrastive decision:
-- membership = cos(image, positive prototype) - cos(image, background/negative
-- prototype) >= threshold, i.e. argmax over {positive, background}. The
-- threshold is now a RELATIVE margin, not an absolute cosine/probability.
-- 0.0 = pure argmax (positive must beat background); tune upward per classifier
-- via Preview for more precision. Negative prototypes are rebuilt by
-- EnsurePrototypes on the next run.
UPDATE classifier_definitions
SET threshold = 0.0,
    updated_at = NOW();
