-- Create "users" table
CREATE TABLE "users" (
  "user_id" serial NOT NULL,
  "username" character varying(50) NOT NULL,
  "email" character varying(100) NOT NULL,
  "password" character varying(255) NOT NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "is_active" boolean NULL DEFAULT true,
  "last_login" timestamptz NULL,
  PRIMARY KEY ("user_id"),
  CONSTRAINT "users_email_key" UNIQUE ("email"),
  CONSTRAINT "users_username_key" UNIQUE ("username")
);
-- Create "assets" table
CREATE TABLE "assets" (
  "asset_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "owner_id" integer NULL,
  "type" character varying(20) NOT NULL,
  "original_filename" character varying(255) NOT NULL,
  "storage_path" character varying(50) NOT NULL,
  "mime_type" character varying(50) NOT NULL,
  "file_size" bigint NOT NULL,
  "hash" character varying(64) NULL,
  "width" integer NULL,
  "height" integer NULL,
  "duration" double precision NULL,
  "upload_time" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "is_deleted" boolean NULL DEFAULT false,
  "deleted_at" timestamptz NULL,
  "specific_metadata" jsonb NULL,
  "embedding" vector(768) NULL,
  PRIMARY KEY ("asset_id"),
  CONSTRAINT "assets_owner_id_fkey" FOREIGN KEY ("owner_id") REFERENCES "users" ("user_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "assets_type_check" CHECK ((type)::text = ANY ((ARRAY['PHOTO'::character varying, 'VIDEO'::character varying, 'AUDIO'::character varying])::text[]))
);
-- Create index "assets_hnsw_idx" to table: "assets"
CREATE INDEX "assets_hnsw_idx" ON "assets" USING hnsw ("embedding" vector_l2_ops);
-- Create index "idx_assets_hash" to table: "assets"
CREATE INDEX "idx_assets_hash" ON "assets" ("hash");
-- Create index "idx_assets_owner_id" to table: "assets"
CREATE INDEX "idx_assets_owner_id" ON "assets" ("owner_id");
-- Create index "idx_assets_type" to table: "assets"
CREATE INDEX "idx_assets_type" ON "assets" ("type");
-- Create "albums" table
CREATE TABLE "albums" (
  "album_id" serial NOT NULL,
  "user_id" integer NOT NULL,
  "album_name" character varying(100) NOT NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "description" text NULL,
  "cover_asset_id" uuid NULL,
  PRIMARY KEY ("album_id"),
  CONSTRAINT "albums_cover_asset_id_fkey" FOREIGN KEY ("cover_asset_id") REFERENCES "assets" ("asset_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "albums_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users" ("user_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_albums_user_id" to table: "albums"
CREATE INDEX "idx_albums_user_id" ON "albums" ("user_id");
-- Create "album_assets" table
CREATE TABLE "album_assets" (
  "album_id" integer NOT NULL,
  "asset_id" uuid NOT NULL,
  "position" integer NULL DEFAULT 0,
  "added_time" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("album_id", "asset_id"),
  CONSTRAINT "album_assets_album_id_fkey" FOREIGN KEY ("album_id") REFERENCES "albums" ("album_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "album_assets_asset_id_fkey" FOREIGN KEY ("asset_id") REFERENCES "assets" ("asset_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "tags" table
CREATE TABLE "tags" (
  "tag_id" serial NOT NULL,
  "tag_name" character varying(50) NOT NULL,
  "category" character varying(50) NULL,
  "is_ai_generated" boolean NULL DEFAULT true,
  PRIMARY KEY ("tag_id"),
  CONSTRAINT "tags_tag_name_key" UNIQUE ("tag_name")
);
-- Create "asset_tags" table
CREATE TABLE "asset_tags" (
  "asset_id" uuid NOT NULL,
  "tag_id" integer NOT NULL,
  "confidence" numeric(4,3) NOT NULL,
  "source" character varying(20) NOT NULL DEFAULT 'system',
  PRIMARY KEY ("asset_id", "tag_id"),
  CONSTRAINT "asset_tags_asset_id_fkey" FOREIGN KEY ("asset_id") REFERENCES "assets" ("asset_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "asset_tags_tag_id_fkey" FOREIGN KEY ("tag_id") REFERENCES "tags" ("tag_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "asset_tags_confidence_check" CHECK ((confidence >= (0)::numeric) AND (confidence <= (1)::numeric)),
  CONSTRAINT "asset_tags_source_check" CHECK ((source)::text = ANY ((ARRAY['system'::character varying, 'user'::character varying, 'ai'::character varying])::text[]))
);
-- Create "refresh_tokens" table
CREATE TABLE "refresh_tokens" (
  "token_id" serial NOT NULL,
  "user_id" integer NOT NULL,
  "token" character varying(255) NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "is_revoked" boolean NULL DEFAULT false,
  PRIMARY KEY ("token_id"),
  CONSTRAINT "refresh_tokens_token_key" UNIQUE ("token"),
  CONSTRAINT "refresh_tokens_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users" ("user_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_refresh_tokens_tokens_token" to table: "refresh_tokens"
CREATE INDEX "idx_refresh_tokens_tokens_token" ON "refresh_tokens" ("token");
-- Create index "idx_refresh_tokens_user_id" to table: "refresh_tokens"
CREATE INDEX "idx_refresh_tokens_user_id" ON "refresh_tokens" ("user_id");
-- Create "species_predictions" table
CREATE TABLE "species_predictions" (
  "asset_id" uuid NOT NULL,
  "label" character varying(255) NOT NULL,
  "score" real NOT NULL,
  PRIMARY KEY ("asset_id", "label"),
  CONSTRAINT "species_predictions_asset_id_fkey" FOREIGN KEY ("asset_id") REFERENCES "assets" ("asset_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_species_predictions_asset_id" to table: "species_predictions"
CREATE INDEX "idx_species_predictions_asset_id" ON "species_predictions" ("asset_id");
-- Create "thumbnails" table
CREATE TABLE "thumbnails" (
  "thumbnail_id" serial NOT NULL,
  "asset_id" uuid NOT NULL,
  "size" character varying(20) NOT NULL,
  "storage_path" character varying(512) NOT NULL,
  "mime_type" character varying(50) NOT NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("thumbnail_id"),
  CONSTRAINT "thumbnails_asset_id_fkey" FOREIGN KEY ("asset_id") REFERENCES "assets" ("asset_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "thumbnails_size_check" CHECK ((size)::text = ANY ((ARRAY['small'::character varying, 'medium'::character varying, 'large'::character varying])::text[]))
);
-- Create index "idx_thumbnails_asset_id" to table: "thumbnails"
CREATE INDEX "idx_thumbnails_asset_id" ON "thumbnails" ("asset_id");
