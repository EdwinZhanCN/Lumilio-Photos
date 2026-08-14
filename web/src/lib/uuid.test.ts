import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { createUUID, installRandomUUIDCompatibility } from "./uuid";

describe("createUUID", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses the native randomUUID implementation when available", () => {
    const randomUUID = vi.fn(() => "123e4567-e89b-42d3-a456-426614174000");
    const getRandomValues = vi.fn();
    vi.stubGlobal("crypto", { randomUUID, getRandomValues });

    expect(createUUID()).toBe("123e4567-e89b-42d3-a456-426614174000");
    expect(randomUUID).toHaveBeenCalledOnce();
    expect(getRandomValues).not.toHaveBeenCalled();
  });

  it("builds a version 4 UUID from getRandomValues when randomUUID is unavailable", () => {
    const getRandomValues = vi.fn((target: Uint8Array) => {
      target.set([0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15]);
      return target;
    });
    vi.stubGlobal("crypto", { getRandomValues });

    expect(createUUID()).toBe("00010203-0405-4607-8809-0a0b0c0d0e0f");
    expect(getRandomValues).toHaveBeenCalledOnce();
  });

  it("fails explicitly when secure random generation is unavailable", () => {
    vi.stubGlobal("crypto", undefined);

    expect(() => createUUID()).toThrow("Secure random UUID generation is unavailable");
  });

  it("installs a randomUUID compatibility method for browser dependencies", () => {
    const getRandomValues = vi.fn((target: Uint8Array) => {
      target.fill(0xab);
      return target;
    });
    const cryptoAPI = { getRandomValues };

    expect(installRandomUUIDCompatibility(cryptoAPI)).toBe(true);
    expect(cryptoAPI).toHaveProperty("randomUUID");
    expect((cryptoAPI as typeof cryptoAPI & { randomUUID: () => string }).randomUUID()).toBe(
      "abababab-abab-4bab-abab-abababababab",
    );
  });

  it("does not replace a native randomUUID implementation", () => {
    const randomUUID = vi.fn(() => "123e4567-e89b-42d3-a456-426614174000" as const);
    const cryptoAPI = { getRandomValues: vi.fn(), randomUUID };

    expect(installRandomUUIDCompatibility(cryptoAPI)).toBe(true);
    expect(cryptoAPI.randomUUID()).toBe("123e4567-e89b-42d3-a456-426614174000");
    expect(randomUUID).toHaveBeenCalledOnce();
  });
});
