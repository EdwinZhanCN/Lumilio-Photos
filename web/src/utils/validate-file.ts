/**
 * Validates if a file is of a supported type
 * @param {File} file - The file to validate
 * @returns {boolean} - True if the file is of a supported type
 */
const isValidFileType = (file:File):boolean => {
    const supportedImageTypes = [
        'image/',
        'image/x-canon-cr2',
        'image/x-nikon-nef',
        'image/x-sony-arw',
        'image/x-adobe-dng',
        'image/x-fuji-raf',
        'image/x-panasonic-rw2'
    ];

    const supportedVideoTypes = [
        'video/mp4',
        'video/quicktime',
        'video/x-msvideo',
        'video/x-matroska',
        'video/avi',
        'video/mpeg'
    ];

    // Check if it's an image type that starts with any of the supported prefixes
    const isImage = supportedImageTypes.some(type => file.type.startsWith(type));

    // Check if it's a supported video type
    const isVideo = supportedVideoTypes.includes(file.type);

    return isImage || isVideo;
};

/**
 * Returns an array of supported raw file extensions
 * @returns {string[]} Array of supported raw file extensions
 */
export const getSupportedRawExtensions = (): string[] => {
    return ['.cr2', '.nef', '.arw', '.raf', '.rw2', '.dng'];
};

/**
 * Returns a string of all supported file extensions for accept attribute
 * @returns {string} Comma-separated string of file extensions
 */
export const getSupportedFileExtensionsString = (): string => {
    const rawExtensions = getSupportedRawExtensions().join(',');
    return `image/*,video/*,${rawExtensions},.mov,.mp4,.avi,.mkv`;
};

export default isValidFileType;