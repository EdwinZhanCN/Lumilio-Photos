/**
 * Supported file extensions based on backend definitions
 * Reference: server/internal/utils/file/validator.go
 */

// Standard photo/image formats
export const supportedPhotoExtensions = [
  ".jpg",
  ".jpeg",
  ".png",
  ".webp",
  ".gif",
  ".bmp",
  ".tiff",
  ".tif",
  ".heic",
  ".heif",
] as const;

// RAW camera formats
export const supportedRAWExtensions = [
  ".cr2", // Canon RAW 2
  ".cr3", // Canon RAW 3
  ".nef", // Nikon Electronic Format
  ".arw", // Sony Alpha RAW
  ".dng", // Adobe Digital Negative
  ".orf", // Olympus RAW Format
  ".rw2", // Panasonic RAW 2
  ".pef", // Pentax Electronic Format
  ".raf", // Fujifilm RAW Format
  ".mrw", // Minolta RAW
  ".srw", // Samsung RAW
  ".rwl", // Leica RAW
  ".x3f", // Sigma RAW Format
] as const;

// Video formats
export const supportedVideoExtensions = [
  ".mp4",
  ".mov",
  ".avi",
  ".mkv",
  ".webm",
  ".flv",
  ".wmv",
  ".m4v",
  ".3gp",
  ".mpg",
  ".mpeg",
  ".m2ts",
  ".mts",
  ".ogv",
] as const;

// Audio formats
export const supportedAudioExtensions = [
  ".mp3",
  ".aac",
  ".m4a",
  ".flac",
  ".wav",
  ".ogg",
  ".aiff",
  ".wma",
  ".opus",
  ".oga",
] as const;

// Combined list of all supported extensions
export const acceptFileExtensions = [
  ...supportedPhotoExtensions,
  ...supportedRAWExtensions,
  ...supportedVideoExtensions,
  ...supportedAudioExtensions,
] as const;

// Type definitions
export type PhotoExtension = (typeof supportedPhotoExtensions)[number];
export type RAWExtension = (typeof supportedRAWExtensions)[number];
export type VideoExtension = (typeof supportedVideoExtensions)[number];
export type AudioExtension = (typeof supportedAudioExtensions)[number];
export type SupportedExtension = (typeof acceptFileExtensions)[number];

// Helper functions
export const isPhotoExtension = (ext: string): boolean => {
  return supportedPhotoExtensions.includes(ext.toLowerCase() as PhotoExtension);
};

export const isRAWExtension = (ext: string): boolean => {
  return supportedRAWExtensions.includes(ext.toLowerCase() as RAWExtension);
};

export const isVideoExtension = (ext: string): boolean => {
  return supportedVideoExtensions.includes(ext.toLowerCase() as VideoExtension);
};

export const isAudioExtension = (ext: string): boolean => {
  return supportedAudioExtensions.includes(ext.toLowerCase() as AudioExtension);
};

export const isSupportedExtension = (ext: string): boolean => {
  return acceptFileExtensions.includes(ext.toLowerCase() as SupportedExtension);
};

/**
 * Get file extension from filename
 */
export const getFileExtension = (filename: string): string => {
  const ext = filename.toLowerCase().match(/\.[^.]+$/)?.[0];
  return ext || "";
};

/**
 * Get asset type from file extension
 */
export const getAssetTypeFromExtension = (
  filename: string,
): "photo" | "video" | "audio" | "unknown" => {
  const ext = getFileExtension(filename);

  if (isPhotoExtension(ext) || isRAWExtension(ext)) {
    return "photo";
  }
  if (isVideoExtension(ext)) {
    return "video";
  }
  if (isAudioExtension(ext)) {
    return "audio";
  }

  return "unknown";
};

/**
 * Get human-readable format name
 */
export const getFormatName = (ext: string): string => {
  const extLower = ext.toLowerCase().replace(".", "");

  // Special cases
  const formatNames: Record<string, string> = {
    jpg: "JPEG",
    jpeg: "JPEG",
    tif: "TIFF",
    tiff: "TIFF",
    heic: "HEIC",
    heif: "HEIF",
    cr2: "Canon RAW (CR2)",
    cr3: "Canon RAW (CR3)",
    nef: "Nikon RAW (NEF)",
    arw: "Sony RAW (ARW)",
    dng: "Adobe DNG",
    orf: "Olympus RAW",
    rw2: "Panasonic RAW",
    pef: "Pentax RAW",
    raf: "Fujifilm RAW",
    mrw: "Minolta RAW",
    srw: "Samsung RAW",
    rwl: "Leica RAW",
    x3f: "Sigma RAW",
    mp4: "MP4",
    mov: "QuickTime",
    avi: "AVI",
    mkv: "Matroska",
    webm: "WebM",
    m4v: "iTunes Video",
    "3gp": "3GPP",
    mpg: "MPEG",
    mpeg: "MPEG",
    m2ts: "MPEG-2 TS",
    mts: "AVCHD",
    mp3: "MP3",
    aac: "AAC",
    m4a: "M4A",
    flac: "FLAC",
    wav: "WAV",
    ogg: "Ogg Vorbis",
    aiff: "AIFF",
    wma: "WMA",
    opus: "Opus",
  };

  return formatNames[extLower] || extLower.toUpperCase();
};

/**
 * Get accept attribute value for file input
 */
export const getAcceptString = (): string => {
  return acceptFileExtensions.join(",");
};

/**
 * Get grouped formats for display
 */
export interface FormatGroup {
  category: string;
  formats: Array<{ ext: string; name: string }>;
}

export const getFormatGroups = (): FormatGroup[] => {
  return [
    {
      category: "Photos",
      formats: supportedPhotoExtensions.map((ext) => ({
        ext,
        name: getFormatName(ext),
      })),
    },
    {
      category: "RAW Formats",
      formats: supportedRAWExtensions.map((ext) => ({
        ext,
        name: getFormatName(ext),
      })),
    },
    {
      category: "Videos",
      formats: supportedVideoExtensions.map((ext) => ({
        ext,
        name: getFormatName(ext),
      })),
    },
    {
      category: "Audio",
      formats: supportedAudioExtensions.map((ext) => ({
        ext,
        name: getFormatName(ext),
      })),
    },
  ];
};

/**
 * Get supported formats summary for display
 */
export const getSupportedFormatsSummary = (): string => {
  const counts = {
    photos: supportedPhotoExtensions.length,
    raw: supportedRAWExtensions.length,
    videos: supportedVideoExtensions.length,
    audio: supportedAudioExtensions.length,
  };

  return `${counts.photos + counts.raw} image formats, ${counts.videos} video formats, ${counts.audio} audio formats`;
};
