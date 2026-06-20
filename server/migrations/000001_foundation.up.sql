-- Squashed baseline migration. Destructive reset: historical migrations were intentionally removed.
-- Extensions and global schema objects required by Lumilio Photos.
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

--
-- Name: album_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.album_type AS ENUM (
    'default',
    'bio'
);


--
-- Name: stack_relation; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.stack_relation AS ENUM (
    'raw_original',
    'jpeg_original',
    'edited_version',
    'alternative',
    'live_photo_still',
    'live_photo_video'
);


--
-- Name: set_assets_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_assets_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


--
-- Name: set_location_clusters_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_location_clusters_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


--
-- Name: update_cluster_member_count(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_cluster_member_count() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE face_clusters
        SET member_count = member_count + 1,
            updated_at = CURRENT_TIMESTAMP
        WHERE cluster_id = NEW.cluster_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE face_clusters
        SET member_count = GREATEST(member_count - 1, 0),
            updated_at = CURRENT_TIMESTAMP
        WHERE cluster_id = OLD.cluster_id;
        RETURN OLD;
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.cluster_id IS DISTINCT FROM NEW.cluster_id THEN
            UPDATE face_clusters
            SET member_count = GREATEST(member_count - 1, 0),
                updated_at = CURRENT_TIMESTAMP
            WHERE cluster_id = OLD.cluster_id;

            UPDATE face_clusters
            SET member_count = member_count + 1,
                updated_at = CURRENT_TIMESTAMP
            WHERE cluster_id = NEW.cluster_id;
        END IF;
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$;


--
-- Name: update_face_results_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_face_results_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE face_results
    SET updated_at = CURRENT_TIMESTAMP
    WHERE asset_id = NEW.asset_id;
    RETURN NEW;
END;
$$;


--
-- Name: update_ocr_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_ocr_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE ocr_results
    SET updated_at = CURRENT_TIMESTAMP
    WHERE asset_id = NEW.asset_id;
    RETURN NEW;
END;
$$;


