import { describe, expect, it } from "vitest";
import { parseTimedLyrics, activeLyricIndex } from "./lyrics";

describe("local LRC playback", () => {
  it("sorts repeated timestamps and applies offsets including fractions", () => {
    const lines = parseTimedLyrics(
      "[ar:Artist]\n[offset:+500]\n[00:02.50][00:04.125]Chorus\n[00:00.2]Intro\n[00:99]Invalid",
    );
    expect(lines).toEqual([
      { time: 0, text: "Intro" },
      { time: 2, text: "Chorus" },
      { time: 3.625, text: "Chorus" },
    ]);
    expect(activeLyricIndex(lines, 1.9)).toBe(0);
    expect(activeLyricIndex(lines, 2)).toBe(1);
    expect(activeLyricIndex(lines, 4)).toBe(2);
  });
  it("retains plain text fallback and has no active line before the first timestamp", () => {
    expect(parseTimedLyrics("Plain lyrics\n第二行")).toEqual([]);
    expect(activeLyricIndex(parseTimedLyrics("[01:00]Later"), 20)).toBe(-1);
    expect(parseTimedLyrics("[offset:+1000]\n[00:02]Text")[0].time).toBe(1);
  });
});
