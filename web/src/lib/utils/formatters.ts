/**
 * Formats bytes into a human-readable string (KB, MB, GB, etc.)
 * @param {number} bytes - The number of bytes to format
 * @param {number} [decimals=2] - Number of decimal places to include
 * @returns {string} Formatted string representation of the byte size
 */
export const formatBytes = (bytes: number, decimals: number = 2): string => {
  if (bytes === 0) return "0 Bytes";

  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];

  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
};
