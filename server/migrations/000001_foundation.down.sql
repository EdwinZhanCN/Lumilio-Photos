DROP TEXT SEARCH CONFIGURATION IF EXISTS public.chinese_zh;

DROP FUNCTION IF EXISTS public.update_ocr_updated_at();
DROP FUNCTION IF EXISTS public.update_face_results_updated_at();
DROP FUNCTION IF EXISTS public.update_cluster_member_count();
DROP FUNCTION IF EXISTS public.set_location_clusters_updated_at();
DROP FUNCTION IF EXISTS public.set_assets_updated_at();

DROP TYPE IF EXISTS public.stack_relation;
DROP TYPE IF EXISTS public.album_type;

DROP EXTENSION IF EXISTS zhparser;
DROP EXTENSION IF EXISTS pg_textsearch;
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS vector;
