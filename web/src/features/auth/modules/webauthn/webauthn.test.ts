import type { TFunction } from "i18next";
import { describe, expect, it, vi } from "vite-plus/test";
import {
  getPasskeySupport,
  getPasskeySupportMessage,
  type PasskeySupportEnvironment,
  type PasskeySupportReasonKey,
} from "./webauthn";

const secureBrowser: PasskeySupportEnvironment = {
  browser: true,
  secureContext: true,
  publicKeyCredentialAvailable: true,
  protocol: "https:",
  hostname: "photos.example.com",
};

describe("getPasskeySupport", () => {
  it("reports browser-only support outside a browser", () => {
    expect(getPasskeySupport({ ...secureBrowser, browser: false })).toEqual({
      supported: false,
      reasonKey: "auth.passkeySupport.browserOnly",
    });
  });

  it("reports the secure-context requirement before browser API support", () => {
    expect(
      getPasskeySupport({
        ...secureBrowser,
        secureContext: false,
        publicKeyCredentialAvailable: false,
        protocol: "http:",
        hostname: "192.168.1.100",
      }),
    ).toEqual({
      supported: false,
      reasonKey: "auth.passkeySupport.secureContextRequired",
    });
  });

  it("reports missing WebAuthn support in a secure browser", () => {
    expect(getPasskeySupport({ ...secureBrowser, publicKeyCredentialAvailable: false })).toEqual({
      supported: false,
      reasonKey: "auth.passkeySupport.notSupported",
    });
  });

  it("supports WebAuthn over HTTPS and localhost", () => {
    expect(getPasskeySupport(secureBrowser)).toEqual({ supported: true });
    expect(
      getPasskeySupport({
        ...secureBrowser,
        protocol: "http:",
        hostname: "localhost",
      }),
    ).toEqual({ supported: true });
  });
});

describe("getPasskeySupportMessage", () => {
  const defaults: Record<PasskeySupportReasonKey, string> = {
    "auth.passkeySupport.browserOnly": "Passkeys are available only in a browser.",
    "auth.passkeySupport.notSupported": "Passkeys are not supported by this browser.",
    "auth.passkeySupport.secureContextRequired": "Passkeys require HTTPS or localhost.",
    "auth.passkeySupport.httpsRequired": "Passkeys require HTTPS or a localhost address.",
  };

  it("provides an extractable default for every reason", () => {
    const t = vi.fn((key: string, options?: { defaultValue?: string }) => {
      return options?.defaultValue ?? key;
    }) as unknown as TFunction;

    for (const [reasonKey, expected] of Object.entries(defaults)) {
      expect(getPasskeySupportMessage(t, reasonKey as PasskeySupportReasonKey)).toBe(expected);
    }
    expect(t).toHaveBeenCalledTimes(Object.keys(defaults).length);
  });
});
