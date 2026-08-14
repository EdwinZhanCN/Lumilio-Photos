import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { ClipboardUnavailableError, copyText, isAsyncClipboardAvailable } from "./clipboard";

describe("copyText", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uses the asynchronous Clipboard API when it is available", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    expect(isAsyncClipboardAvailable()).toBe(true);
    await copyText("hello");
    expect(writeText).toHaveBeenCalledWith("hello");
  });

  it("uses execCommand when the secure-context Clipboard API is unavailable", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
    const execCommand = vi.fn(() => true);
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: execCommand,
    });

    expect(isAsyncClipboardAvailable()).toBe(false);
    await copyText("LAN HTTP text");
    expect(execCommand).toHaveBeenCalledWith("copy");
    expect(document.querySelector("textarea[aria-hidden='true']")).toBeNull();
  });

  it("falls back after an asynchronous clipboard permission failure", async () => {
    const writeText = vi.fn().mockRejectedValue(new DOMException("Denied", "NotAllowedError"));
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    const execCommand = vi.fn(() => true);
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: execCommand,
    });

    await copyText("fallback");
    expect(writeText).toHaveBeenCalledOnce();
    expect(execCommand).toHaveBeenCalledWith("copy");
  });

  it("fails explicitly when neither copy mechanism succeeds", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: vi.fn(() => false),
    });

    await expect(copyText("no copy")).rejects.toBeInstanceOf(ClipboardUnavailableError);
  });
});
