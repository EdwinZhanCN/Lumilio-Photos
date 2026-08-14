import type { TFunction } from "i18next";

export type PasskeySupportReasonKey =
  | "auth.passkeySupport.browserOnly"
  | "auth.passkeySupport.notSupported"
  | "auth.passkeySupport.secureContextRequired"
  | "auth.passkeySupport.httpsRequired";

export interface PasskeySupport {
  supported: boolean;
  reasonKey?: PasskeySupportReasonKey;
}

export type PasskeySupportEnvironment = {
  browser: boolean;
  secureContext: boolean;
  publicKeyCredentialAvailable: boolean;
  protocol: string;
  hostname: string;
};

function getBrowserEnvironment(): PasskeySupportEnvironment {
  const browser = typeof window !== "undefined";
  return {
    browser,
    secureContext: browser && window.isSecureContext,
    publicKeyCredentialAvailable: browser && "PublicKeyCredential" in window,
    protocol: browser ? window.location.protocol : "",
    hostname: browser ? window.location.hostname : "",
  };
}

function isLocalHostname(hostname: string) {
  return hostname === "localhost" || hostname === "127.0.0.1";
}

export function getPasskeySupport(
  environment: PasskeySupportEnvironment = getBrowserEnvironment(),
): PasskeySupport {
  if (!environment.browser) {
    return {
      supported: false,
      reasonKey: "auth.passkeySupport.browserOnly",
    };
  }

  if (!environment.secureContext) {
    return {
      supported: false,
      reasonKey: "auth.passkeySupport.secureContextRequired",
    };
  }

  if (!environment.publicKeyCredentialAvailable) {
    return {
      supported: false,
      reasonKey: "auth.passkeySupport.notSupported",
    };
  }

  if (environment.protocol === "https:" || isLocalHostname(environment.hostname)) {
    return { supported: true };
  }

  return {
    supported: false,
    reasonKey: "auth.passkeySupport.httpsRequired",
  };
}

export function getPasskeySupportMessage(
  t: TFunction,
  reasonKey: PasskeySupportReasonKey | undefined,
): string | null {
  switch (reasonKey) {
    case "auth.passkeySupport.browserOnly":
      return t("auth.passkeySupport.browserOnly", {
        defaultValue: "Passkeys are available only in a browser.",
      });
    case "auth.passkeySupport.notSupported":
      return t("auth.passkeySupport.notSupported", {
        defaultValue: "Passkeys are not supported by this browser.",
      });
    case "auth.passkeySupport.secureContextRequired":
      return t("auth.passkeySupport.secureContextRequired", {
        defaultValue: "Passkeys require HTTPS or localhost.",
      });
    case "auth.passkeySupport.httpsRequired":
      return t("auth.passkeySupport.httpsRequired", {
        defaultValue: "Passkeys require HTTPS or a localhost address.",
      });
    default:
      return null;
  }
}

function base64UrlToUint8Array(value: string): Uint8Array {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
  const binary = window.atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

function coerceBinaryValue(value: unknown, fieldName: string): Uint8Array {
  if (typeof value === "string") {
    return base64UrlToUint8Array(value);
  }

  if (Array.isArray(value)) {
    return Uint8Array.from(value);
  }

  if (value instanceof ArrayBuffer) {
    return new Uint8Array(value);
  }

  if (ArrayBuffer.isView(value)) {
    return new Uint8Array(value.buffer, value.byteOffset, value.byteLength);
  }

  throw new Error(`Invalid passkey payload for ${fieldName}.`);
}

function bufferSourceToBase64Url(value: ArrayBuffer | ArrayBufferView | null) {
  if (!value) return undefined;

  const bytes =
    value instanceof ArrayBuffer
      ? new Uint8Array(value)
      : new Uint8Array(value.buffer, value.byteOffset, value.byteLength);

  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });

  return window.btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function coerceCreationOptions(payload: unknown): PublicKeyCredentialCreationOptions {
  const source = payload as {
    response?: Record<string, unknown>;
    publicKey?: Record<string, unknown>;
  };
  const options = (source?.publicKey ?? source?.response ?? payload) as Record<string, unknown>;
  const user = (options.user ?? {}) as Record<string, unknown>;

  return {
    ...options,
    challenge: coerceBinaryValue(options.challenge, "challenge"),
    user: {
      ...user,
      id: coerceBinaryValue(user.id, "user.id"),
    },
    excludeCredentials: Array.isArray(options.excludeCredentials)
      ? options.excludeCredentials.map((credential) => ({
          ...(credential as Record<string, unknown>),
          id: coerceBinaryValue(
            (credential as Record<string, unknown>).id,
            "excludeCredentials.id",
          ),
        }))
      : undefined,
  } as unknown as PublicKeyCredentialCreationOptions;
}

function coerceRequestOptions(payload: unknown): PublicKeyCredentialRequestOptions {
  const source = payload as {
    response?: Record<string, unknown>;
    publicKey?: Record<string, unknown>;
  };
  const options = (source?.publicKey ?? source?.response ?? payload) as Record<string, unknown>;

  return {
    ...options,
    challenge: coerceBinaryValue(options.challenge, "challenge"),
    allowCredentials: Array.isArray(options.allowCredentials)
      ? options.allowCredentials.map((credential) => ({
          ...(credential as Record<string, unknown>),
          id: coerceBinaryValue((credential as Record<string, unknown>).id, "allowCredentials.id"),
        }))
      : undefined,
  } as unknown as PublicKeyCredentialRequestOptions;
}

function serializeCredential(credential: Credential | null) {
  const publicKeyCredential = credential as PublicKeyCredential | null;
  if (!publicKeyCredential) {
    throw new Error("auth.passkeySupport.operationCancelled");
  }

  const response = publicKeyCredential.response;
  const clientExtensionResults = publicKeyCredential.getClientExtensionResults?.() ?? {};

  if (response instanceof AuthenticatorAttestationResponse) {
    return {
      id: publicKeyCredential.id,
      rawId: bufferSourceToBase64Url(publicKeyCredential.rawId),
      type: publicKeyCredential.type,
      authenticatorAttachment: publicKeyCredential.authenticatorAttachment ?? undefined,
      clientExtensionResults,
      response: {
        clientDataJSON: bufferSourceToBase64Url(response.clientDataJSON),
        attestationObject: bufferSourceToBase64Url(response.attestationObject),
        transports: response.getTransports?.() ?? undefined,
      },
    };
  }

  if (response instanceof AuthenticatorAssertionResponse) {
    return {
      id: publicKeyCredential.id,
      rawId: bufferSourceToBase64Url(publicKeyCredential.rawId),
      type: publicKeyCredential.type,
      authenticatorAttachment: publicKeyCredential.authenticatorAttachment ?? undefined,
      clientExtensionResults,
      response: {
        clientDataJSON: bufferSourceToBase64Url(response.clientDataJSON),
        authenticatorData: bufferSourceToBase64Url(response.authenticatorData),
        signature: bufferSourceToBase64Url(response.signature),
        userHandle: bufferSourceToBase64Url(response.userHandle),
      },
    };
  }

  throw new Error("auth.passkeySupport.unsupportedResponse");
}

export async function createPasskeyCredential(options: unknown) {
  const credential = await navigator.credentials.create({
    publicKey: coerceCreationOptions(options),
  });

  return serializeCredential(credential);
}

export async function getPasskeyCredential(options: unknown) {
  const credential = await navigator.credentials.get({
    publicKey: coerceRequestOptions(options),
  });

  return serializeCredential(credential);
}
