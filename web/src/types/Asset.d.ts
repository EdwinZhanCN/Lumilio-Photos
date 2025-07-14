interface Asset {
  albums?: AssetAlbum[];
  assetId?: string;
  deletedAt?: string;
  duration?: number; // For video/audio
  fileSize?: number;
  hash?: string;
  height?: number;
  isDeleted?: boolean;
  mimeType?: string;
  originalFilename?: string;
  ownerId?: number;
  specificMetadata?: JSON;
  storagePath?: string;
  tags?: AssetTag[];
  thumbnails?: AssetThumbnail[];
  type?: "PHOTO" | "VIDEO" | "AUDIO" | "DOCUMENT";
  uploadTime?: string;
  width?: number;
}

interface AssetAlbum {
  albumId?: number;
  albumName?: string;
  assets?: Asset[];
  coverAsset?: Asset;
  coverAssetId?: string;
  createdAt?: string;
  description?: string;
  updatedAt?: string;
  userId?: number;
}

interface AssetTag {
  assets?: Asset[];
  category?: string;
  isAiGenerated?: boolean;
  tagId?: number;
  tagName?: string;
}

interface AssetThumbnail {
  assetId?: string;
  createdAt?: string;
  mimeType?: string; // for video thumbnails
  size?: "small" | "medium" | "large";
  storagePath?: string;
  thumbnailId?: number;
}
