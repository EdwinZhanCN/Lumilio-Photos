import { describe, expect, it } from "vitest";
import { validateRepositoryName } from "./repositorySetup";

describe("validateRepositoryName", () => {
  it.each(["家庭媒体", "Family Media 2026", "Media_2026-Archive"])(
    "accepts portable name %s",
    (value) => {
      expect(validateRepositoryName(value)).toBeNull();
    },
  );

  it.each([
    ["", "required"],
    [" Family", "leadingOrTrailingSpace"],
    ["Family ", "leadingOrTrailingSpace"],
    ["Family/Media", "unsupportedCharacter"],
    [String.raw`Family\Media`, "unsupportedCharacter"],
    ["Family.Media", "unsupportedCharacter"],
    ["Family (Media)", "unsupportedCharacter"],
    ["a".repeat(81), "tooManyCharacters"],
    ["𐐀".repeat(61), "tooManyBytes"],
  ] as const)("rejects %j as %s", (value, error) => {
    expect(validateRepositoryName(value)).toBe(error);
  });
});
