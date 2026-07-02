/**
 * Folder identity for `/collections/folders/:folderKey` routes.
 *
 * There is no folder entity or ID: a folder is a (repository, path) pair
 * derived from `storage_path` prefixes. Route params must be a single path
 * segment, so the pair is packed into a URL-safe base64 token instead of
 * exposing raw slashes from the folder path.
 */
export interface FolderIdentity {
  repositoryId: string;
  folderPath: string;
}

function toBase64Url(input: string): string {
  const base64 = btoa(unescape(encodeURIComponent(input)));
  return base64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function fromBase64Url(input: string): string {
  const padded = input.replace(/-/g, "+").replace(/_/g, "/");
  const padding = padded.length % 4 === 0 ? "" : "=".repeat(4 - (padded.length % 4));
  return decodeURIComponent(escape(atob(padded + padding)));
}

export function encodeFolderKey({ repositoryId, folderPath }: FolderIdentity): string {
  return toBase64Url(JSON.stringify({ r: repositoryId, p: folderPath }));
}

export function decodeFolderKey(key: string | undefined): FolderIdentity | null {
  if (!key) return null;
  try {
    const parsed = JSON.parse(fromBase64Url(key)) as { r?: string; p?: string };
    if (!parsed.r) return null;
    return { repositoryId: parsed.r, folderPath: parsed.p ?? "" };
  } catch {
    return null;
  }
}
