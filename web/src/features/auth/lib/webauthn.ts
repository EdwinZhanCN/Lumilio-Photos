export type PasskeySupport = {
  supported: boolean;
  reason?: string;
};

function isLocalHostname(hostname: string) {
  return hostname === "localhost" || hostname === "127.0.0.1";
}

export function getPasskeySupport(): PasskeySupport {
  if (typeof window === "undefined") {
    return {
      supported: false,
      reason: "Passkeys are only available in a browser context.",
    };
  }

  if (!("PublicKeyCredential" in window)) {
    return {
      supported: false,
      reason: "This browser or device does not support passkeys.",
    };
  }

  if (!window.isSecureContext) {
    return {
      supported: false,
      reason: "Passkeys require a secure browser context.",
    };
  }

  const { protocol, hostname } = window.location;
  if (protocol === "https:" || isLocalHostname(hostname)) {
    return { supported: true };
  }

  return {
    supported: false,
    reason: "Passkeys require HTTPS outside localhost development.",
  };
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

function coerceBinaryValue(
  value: unknown,
  fieldName: string,
): Uint8Array {
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

  return window
    .btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/g, "");
}

function coerceCreationOptions(payload: unknown): PublicKeyCredentialCreationOptions {
  const source = payload as {
    response?: Record<string, unknown>;
    publicKey?: Record<string, unknown>;
  };
  const options = (source?.publicKey ?? source?.response ?? payload) as Record<
    string,
    unknown
  >;
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
  const options = (source?.publicKey ?? source?.response ?? payload) as Record<
    string,
    unknown
  >;

  return {
    ...options,
    challenge: coerceBinaryValue(options.challenge, "challenge"),
    allowCredentials: Array.isArray(options.allowCredentials)
      ? options.allowCredentials.map((credential) => ({
          ...(credential as Record<string, unknown>),
          id: coerceBinaryValue(
            (credential as Record<string, unknown>).id,
            "allowCredentials.id",
          ),
        }))
      : undefined,
  } as unknown as PublicKeyCredentialRequestOptions;
}

function serializeCredential(credential: Credential | null) {
  const publicKeyCredential = credential as PublicKeyCredential | null;
  if (!publicKeyCredential) {
    throw new Error("Passkey operation was cancelled.");
  }

  const response = publicKeyCredential.response;
  const clientExtensionResults =
    publicKeyCredential.getClientExtensionResults?.() ?? {};

  if (response instanceof AuthenticatorAttestationResponse) {
    return {
      id: publicKeyCredential.id,
      rawId: bufferSourceToBase64Url(publicKeyCredential.rawId),
      type: publicKeyCredential.type,
      authenticatorAttachment:
        publicKeyCredential.authenticatorAttachment ?? undefined,
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
      authenticatorAttachment:
        publicKeyCredential.authenticatorAttachment ?? undefined,
      clientExtensionResults,
      response: {
        clientDataJSON: bufferSourceToBase64Url(response.clientDataJSON),
        authenticatorData: bufferSourceToBase64Url(response.authenticatorData),
        signature: bufferSourceToBase64Url(response.signature),
        userHandle: bufferSourceToBase64Url(response.userHandle),
      },
    };
  }

  throw new Error("Unsupported passkey response.");
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
