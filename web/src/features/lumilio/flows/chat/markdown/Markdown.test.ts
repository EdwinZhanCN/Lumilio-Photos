import { describe, expect, it } from "vite-plus/test";
import { getMarkdownControls } from "./Markdown";

describe("getMarkdownControls", () => {
  it("hides dependency-owned copy controls without the asynchronous Clipboard API", () => {
    expect(getMarkdownControls(false)).toMatchObject({
      code: { copy: false },
      table: { copy: false },
    });
  });

  it("enables dependency-owned copy controls when the asynchronous Clipboard API exists", () => {
    expect(getMarkdownControls(true)).toMatchObject({
      code: { copy: true },
      table: { copy: true },
    });
  });
});
