/**
 * Formats bytes into a human-readable string (KB, MB, GB, etc.)
 * @param {number} bytes - The number of bytes to format
 * @param {number} [decimals=2] - Number of decimal places to include
 * @returns {string} Formatted string representation of the byte size
 */
export const formatBytes = (bytes, decimals = 2) => {
    if (bytes === 0) return '0 Bytes';

    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

    const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k));

    return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + ' ' + sizes[i];
};

/**
 * Formats a duration in milliseconds to a human-readable time string
 * @param {number} ms - Duration in milliseconds
 * @returns {string} Formatted time string
 */
export const formatDuration = (ms) => {
    if (ms < 1000) return `${ms.toFixed(0)}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`;

    const minutes = Math.floor(ms / 60000);
    const seconds = ((ms % 60000) / 1000).toFixed(1);
    return `${minutes}m ${seconds}s`;
};