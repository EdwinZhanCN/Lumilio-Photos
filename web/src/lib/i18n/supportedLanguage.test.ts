import { describe, expect, it } from "vite-plus/test";
import { toSupportedLanguage } from "../i18n";

describe("toSupportedLanguage", () => {
  it("collapses detected tags to a supported, Intl-valid language", () => {
    expect(toSupportedLanguage("en-US@posix")).toBe("en");
    expect(toSupportedLanguage("zh-Hans-CN")).toBe("zh");
    expect(toSupportedLanguage("zh_TW")).toBe("zh");
    expect(toSupportedLanguage("fr-FR")).toBe("en");
    expect(toSupportedLanguage("")).toBe("en");
    expect(toSupportedLanguage(undefined)).toBe("en");
  });

  it("always yields a tag that Intl accepts", () => {
    for (const detected of ["en-US@posix", "C.UTF-8", "zh-CN", "xx"]) {
      expect(() => new Date(0).toLocaleTimeString(toSupportedLanguage(detected))).not.toThrow();
    }
  });
});
