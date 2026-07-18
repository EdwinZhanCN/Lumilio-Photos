/** Triggers a browser save for an in-memory blob. POST-fetched responses have
 * no URL a plain `<a href>` can point at). Mirrors the pattern already used
 * for authenticated bulk asset downloads in useSelection.tsx. */
export const triggerBlobDownload = (blob: Blob, filename: string): void => {
  const blobUrl = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = blobUrl;
  link.setAttribute("download", filename);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(blobUrl);
};

export const filenameFromContentDisposition = (
  contentDisposition: string | null,
): string | undefined => {
  if (!contentDisposition) return undefined;
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1]);
  }
  const filenameMatch = contentDisposition.match(/filename="?([^"]+)"?/i);
  return filenameMatch?.[1];
};
