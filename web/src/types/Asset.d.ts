interface Asset {
  albums?: AssetAlbum[];
  asset_id?: string;
  deleted_at?: string;
  duration?: number; // For video/audio
  file_size?: number;
  hash?: string;
  height?: number;
  is_deleted?: boolean;
  mime_type?: string;
  original_filename?: string;
  owner_id?: number;
  specific_metadata?: JSON;
  storage_path?: string;
  tags?: AssetTag[];
  thumbnails?: AssetThumbnail[];
  type?: "PHOTO" | "VIDEO" | "AUDIO" | "DOCUMENT";
  upload_time?: string;
  width?: number;
}

interface AssetAlbum {
  album_id?: number;
  album_name?: string;
  assets?: Asset[];
  cover_asset?: Asset;
  cover_asset_id?: string;
  created_at?: string;
  description?: string;
  updated_at?: string;
  user_id?: number;
}

interface AssetTag {
  assets?: Asset[];
  category?: string;
  is_ai_generated?: boolean;
  tag_id?: number;
  tag_name?: string;
}

interface AssetThumbnail {
  asset_id?: string;
  created_at?: string;
  mime_type?: string; // for video thumbnails
  size?: "small" | "medium" | "large";
  storage_path?: string;
  thumbnail_id?: number;
}
