export class ClipboardUnavailableError extends Error {
  constructor() {
    super("Clipboard writing is unavailable");
    this.name = "ClipboardUnavailableError";
  }
}

export function isAsyncClipboardAvailable(): boolean {
  return typeof navigator !== "undefined" && typeof navigator.clipboard?.writeText === "function";
}

function copyTextWithDocument(text: string, targetDocument: Document): boolean {
  if (!targetDocument.body || typeof targetDocument.execCommand !== "function") return false;

  const activeElement =
    targetDocument.activeElement instanceof HTMLElement ? targetDocument.activeElement : null;
  const selection = targetDocument.getSelection();
  const ranges = selection
    ? Array.from({ length: selection.rangeCount }, (_, index) =>
        selection.getRangeAt(index).cloneRange(),
      )
    : [];
  const textarea = targetDocument.createElement("textarea");
  textarea.value = text;
  textarea.readOnly = true;
  textarea.setAttribute("aria-hidden", "true");
  Object.assign(textarea.style, {
    position: "fixed",
    inset: "0 auto auto 0",
    width: "1px",
    height: "1px",
    padding: "0",
    border: "0",
    opacity: "0",
    pointerEvents: "none",
    fontSize: "16px",
  });

  targetDocument.body.append(textarea);
  textarea.focus({ preventScroll: true });
  textarea.select();
  textarea.setSelectionRange(0, textarea.value.length);

  try {
    return targetDocument.execCommand("copy");
  } catch {
    return false;
  } finally {
    textarea.remove();
    selection?.removeAllRanges();
    for (const range of ranges) selection?.addRange(range);
    activeElement?.focus({ preventScroll: true });
  }
}

/** Copy plain text in HTTPS and LAN HTTP browser contexts. */
export async function copyText(text: string): Promise<void> {
  if (isAsyncClipboardAvailable()) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Permission and policy failures can still use the user-gesture fallback.
    }
  }

  if (typeof document !== "undefined" && copyTextWithDocument(text, document)) return;
  throw new ClipboardUnavailableError();
}
