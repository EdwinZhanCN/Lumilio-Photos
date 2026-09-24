import { describe, expect, it } from "vitest";
import { readPlayback, serializePlayback } from "./playbackStorage";

describe("saved music playback", () => {
  it("restores duplicate entries, selection and position without persisting media URLs", () => {
    const raw = serializePlayback({
      queue: [
        { track: { track_id: "a", title: "Track" }, available: true },
        { track: { track_id: "a", title: "Track" }, available: true },
      ],
      currentIndex: 1,
      currentTime: 42,
      volume: 0,
      repeat: "one",
      shuffle: true,
    });
    const restored = readPlayback(raw);
    expect(restored?.queue.map((item) => item.track.track_id)).toEqual(["a", "a"]);
    expect(restored).toMatchObject({
      currentIndex: 1,
      currentTime: 42,
      volume: 0,
      repeat: "one",
      shuffle: true,
    });
    expect(JSON.parse(raw).queue[0]).toEqual({ id: "a", title: "Track" });
  });
  it("rejects corrupt, obsolete and out-of-range snapshots", () => {
    for (const raw of [
      null,
      "{",
      '{"version":0}',
      JSON.stringify({ version: 1, queue: [{ id: 3 }], currentIndex: 0 }),
      JSON.stringify({ version: 1, queue: [{ id: "a" }], currentIndex: 2 }),
    ])
      expect(readPlayback(raw)).toBeNull();
  });
});
