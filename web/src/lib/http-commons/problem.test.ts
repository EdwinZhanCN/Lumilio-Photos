import { createInstance } from "i18next";
import { describe, expect, it } from "vite-plus/test";
import en from "@/locales/en/translation.json";
import zh from "@/locales/zh/translation.json";
import {
  localizeProblem,
  localizeProblemReference,
  normalizeProblem,
  normalizeProblemReference,
} from "./problem";

async function translator(language: "en" | "zh") {
  const instance = createInstance();
  await instance.init({
    lng: language,
    fallbackLng: "en",
    resources: { en: { translation: en }, zh: { translation: zh } },
  });

  return instance.t;
}

describe("Problem Details", () => {
  it("localizes a known type from the current language and ignores injected prose", async () => {
    const normalized = normalizeProblem({
      type: "https://lumilio.org/problems/auth/invalid-credentials",
      status: 401,
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
      title: "INJECTED TITLE",
      detail: "INJECTED DETAIL",
      message: "INJECTED MESSAGE",
      error: "INJECTED ERROR",
    });

    expect(localizeProblem(normalized, await translator("en"), "Sign-in failed.")).toBe(
      "The username or password is incorrect.",
    );
    expect(localizeProblem(normalized, await translator("zh"), "登录失败。")).toBe(
      "用户名或密码不正确。",
    );
  });

  it("interpolates bounded typed extension fields", async () => {
    const value = normalizeProblem({
      type: "https://lumilio.org/problems/auth/rate-limited",
      status: 429,
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
      retry_after_seconds: 12,
    });
    expect(localizeProblem(value, await translator("en"), "Try again.")).toBe(
      "Too many attempts. Try again in 12 seconds.",
    );
    expect(localizeProblem(value, await translator("zh"), "请重试。")).toBe(
      "尝试次数过多，请在 12 秒后重试。",
    );
  });

  it("localizes transport-neutral Problem References", async () => {
    const reference = normalizeProblemReference({
      type: "https://lumilio.org/problems/upload/processing-failed",
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
      retryable: true,
    });
    expect(localizeProblemReference(reference, await translator("en"), "Upload failed.")).toBe(
      "The upload could not be processed.",
    );
    expect(localizeProblemReference(reference, await translator("zh"), "上传失败。")).toBe(
      "无法处理此上传。",
    );
  });

  it("uses the translator's current runtime language", async () => {
    const instance = createInstance();
    await instance.init({
      lng: "en",
      fallbackLng: "en",
      resources: { en: { translation: en }, zh: { translation: zh } },
    });
    const value = normalizeProblem({
      type: "https://lumilio.org/problems/auth/permission-denied",
      status: 403,
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
    });
    expect(localizeProblem(value, instance.t, "Denied.")).toBe(
      "You do not have permission to do that.",
    );
    await instance.changeLanguage("zh");
    expect(localizeProblem(value, instance.t, "已拒绝。")).toBe("你没有执行此操作的权限。");
  });

  it("keeps abort and network failures structured and on the operation fallback", async () => {
    const t = await translator("en");
    expect(normalizeProblem(new DOMException("cancelled", "AbortError"))).toEqual({
      kind: "abort",
    });
    expect(normalizeProblem(new TypeError("offline"))).toEqual({ kind: "network" });
    expect(localizeProblem(new DOMException("cancelled", "AbortError"), t, "Cancelled.")).toBe(
      "Cancelled.",
    );
    expect(localizeProblem(new TypeError("offline"), t, "Check your connection.")).toBe(
      "Check your connection.",
    );
  });

  it.each([
    undefined,
    null,
    {},
    { type: "about:blank", status: 500, instance: "urn:lumilio:problem:x" },
    {
      type: "https://lumilio.org/problems/future/new-problem",
      status: 500,
      instance: "urn:lumilio:problem:0123456789abcdef0123456789abcdef",
    },
  ])("uses the operation fallback for malformed, generic, and future Problems", async (value) => {
    const fallback = "The operation could not be completed.";
    expect(localizeProblem(normalizeProblem(value), await translator("en"), fallback)).toBe(
      fallback,
    );
  });
});
