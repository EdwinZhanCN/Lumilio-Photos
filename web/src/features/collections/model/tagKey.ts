/**
 * Pure tag identity for `/collections/tags/:tagKey` routes.
 *
 * `AssetFilterDTO` filters tags by name + source (not tag ID), and the same
 * tag name can carry both manual and AI/system assignments as distinct
 * browsable entries. The pair is packed into a URL-safe base64 token so tag
 * names containing slashes or spaces stay a single route segment.
 */
export interface TagIdentity {
  tagName: string;
  source: string;
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

export function encodeTagKey({ tagName, source }: TagIdentity): string {
  return toBase64Url(JSON.stringify({ n: tagName, s: source }));
}

export function decodeTagKey(key: string | undefined): TagIdentity | null {
  if (!key) return null;
  try {
    const parsed = JSON.parse(fromBase64Url(key)) as { n?: string; s?: string };
    if (!parsed.n) return null;
    return { tagName: parsed.n, source: parsed.s ?? "" };
  } catch {
    return null;
  }
}
