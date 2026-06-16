-- Zero-shot classifier membership is a cosine floor on the positive-prototype
-- similarity, not a SigLIP probability (those are intrinsically tiny). Re-seed
-- the preset thresholds onto the cosine scale (present concepts on siglip2-base
-- score ≈0.12–0.15; absent ≈0.04–0.09). Tune per classifier via Preview.
UPDATE classifier_definitions
SET threshold = 0.105,
    updated_at = NOW();
