import { createHmac } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

/** Temporary, ignored hand-off between the fresh bootstrap browser and E2E seed. */
export const bootstrapTOTPPath = path.join(repositoryRoot, ".cache/e2e/bootstrap-totp.json");

export type BootstrapTOTP = {
  username: string;
  secret: string;
};

const BASE32_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";

function decodeBase32(value: string): Buffer {
  const normalized = value.toUpperCase().replace(/[^A-Z2-7]/g, "");
  let bits = 0;
  let bitCount = 0;
  const bytes: number[] = [];
  for (const character of normalized) {
    const digit = BASE32_ALPHABET.indexOf(character);
    if (digit < 0) continue;
    bits = (bits << 5) | digit;
    bitCount += 5;
    if (bitCount >= 8) {
      bitCount -= 8;
      bytes.push((bits >> bitCount) & 0xff);
    }
  }
  return Buffer.from(bytes);
}

/** RFC 6238 SHA-1, six digits, thirty-second period (the server contract). */
export function totpCode(secret: string, at = Date.now()): string {
  const counter = Math.floor(at / 1_000 / 30);
  const message = Buffer.alloc(8);
  message.writeBigInt64BE(BigInt(counter));
  const digest = createHmac("sha1", decodeBase32(secret)).update(message).digest();
  const offset = digest[digest.length - 1] & 0x0f;
  const binary =
    ((digest[offset] & 0x7f) << 24) |
    ((digest[offset + 1] & 0xff) << 16) |
    ((digest[offset + 2] & 0xff) << 8) |
    (digest[offset + 3] & 0xff);
  return String(binary % 1_000_000).padStart(6, "0");
}

/** Wait for the next server TOTP counter after a replay rejection. */
export async function nextTOTPCode(secret: string, previous: string): Promise<string> {
  let code = totpCode(secret);
  if (code !== previous) return code;

  const secondsIntoWindow = Math.floor(Date.now() / 1_000) % 30;
  await new Promise((resolve) => setTimeout(resolve, (30 - secondsIntoWindow) * 1_000 + 150));
  code = totpCode(secret);
  return code;
}

export function saveBootstrapTOTP(value: BootstrapTOTP): void {
  mkdirSync(path.dirname(bootstrapTOTPPath), { recursive: true });
  writeFileSync(bootstrapTOTPPath, `${JSON.stringify(value)}\n`, { mode: 0o600 });
}

export function loadBootstrapTOTP(): BootstrapTOTP | null {
  try {
    const parsed: unknown = JSON.parse(readFileSync(bootstrapTOTPPath, "utf8"));
    if (!parsed || typeof parsed !== "object") return null;
    const value = parsed as Partial<BootstrapTOTP>;
    if (typeof value.username !== "string" || typeof value.secret !== "string") return null;
    return value as BootstrapTOTP;
  } catch {
    return null;
  }
}
