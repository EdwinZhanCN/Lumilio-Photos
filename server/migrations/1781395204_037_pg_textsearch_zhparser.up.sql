-- pg_textsearch: native BM25 index access method (requires PostgreSQL 17+).
-- zhparser: Chinese word segmentation via SCWS, plugs into pg_textsearch's
-- text_config parameter for CJK-aware BM25 scoring.

CREATE EXTENSION IF NOT EXISTS pg_textsearch;
CREATE EXTENSION IF NOT EXISTS zhparser;

-- Chinese text search configuration using zhparser's SCWS tokenizer.
-- Token types: n=noun, v=verb, a=adjective, i=idiom, e=email, l=URL, j=abbreviation.
-- The 'simple' dictionary keeps all tokens as-is (no stemming), which is
-- appropriate for OCR text (short phrases, proper nouns, numbers).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_ts_config WHERE cfgname = 'chinese_zh'
    ) THEN
        CREATE TEXT SEARCH CONFIGURATION chinese_zh (PARSER = zhparser);
        ALTER TEXT SEARCH CONFIGURATION chinese_zh
            ADD MAPPING FOR n,v,a,i,e,l,j WITH simple;
    END IF;
END
$$;
