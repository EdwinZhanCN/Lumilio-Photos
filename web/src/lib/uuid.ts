type RandomUUIDCrypto = Pick<Crypto, "getRandomValues"> & {
  randomUUID?: () => `${string}-${string}-${string}-${string}-${string}`;
};

function createUUIDFromRandomValues(cryptoAPI: Pick<Crypto, "getRandomValues">): string {
  if (typeof cryptoAPI?.getRandomValues !== "function") {
    throw new Error("Secure random UUID generation is unavailable");
  }

  const bytes = cryptoAPI.getRandomValues(new Uint8Array(16));
  bytes[6] = ((bytes[6] ?? 0) & 0x0f) | 0x40;
  bytes[8] = ((bytes[8] ?? 0) & 0x3f) | 0x80;
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0"));

  return [
    hex.slice(0, 4).join(""),
    hex.slice(4, 6).join(""),
    hex.slice(6, 8).join(""),
    hex.slice(8, 10).join(""),
    hex.slice(10, 16).join(""),
  ].join("-");
}

/** Generate an RFC 4122 version 4 UUID in secure and LAN HTTP contexts. */
export function createUUID(): string {
  const cryptoAPI = globalThis.crypto as RandomUUIDCrypto | undefined;
  if (typeof cryptoAPI?.randomUUID === "function") return cryptoAPI.randomUUID();
  if (!cryptoAPI) throw new Error("Secure random UUID generation is unavailable");
  return createUUIDFromRandomValues(cryptoAPI);
}

/**
 * Supply the standards-compatible randomUUID surface to browser dependencies on
 * LAN HTTP origins. Web Crypto still provides getRandomValues there, but hides
 * randomUUID because the latter is marked as a secure-context API.
 */
export function installRandomUUIDCompatibility(
  cryptoAPI: RandomUUIDCrypto | undefined = globalThis.crypto as RandomUUIDCrypto | undefined,
): boolean {
  if (!cryptoAPI || typeof cryptoAPI.getRandomValues !== "function") return false;
  if (typeof cryptoAPI.randomUUID === "function") return true;

  try {
    Object.defineProperty(cryptoAPI, "randomUUID", {
      configurable: true,
      value: () => createUUIDFromRandomValues(cryptoAPI),
    });
    return typeof cryptoAPI.randomUUID === "function";
  } catch {
    return false;
  }
}
