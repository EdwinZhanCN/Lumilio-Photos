import { describe, expect, it } from "vitest";
import { nextQueueIndex, playbackEntryToQueueItem, trackArtist, trackTitle } from "./music";

describe("music model helpers", () => {
  it("keeps explicit artist credits ordered without splitting names", () => {
    expect(
      trackArtist({
        artists: [
          { display_name: "A & B", position: 0, role: "track", artist_id: "a" },
          { display_name: "C", position: 1, role: "track", artist_id: "c" },
        ],
        artist_name: "ignored",
      }),
    ).toBe("A & B · C");
  });

  it("uses filename and saved tombstone text as display fallbacks", () => {
    expect(trackTitle({ original_filename: "voice-note.m4a" })).toBe("voice-note.m4a");
    expect(
      playbackEntryToQueueItem({
        entry_id: "entry",
        sequence: 2,
        saved_title: "Deleted recording",
        available: false,
      })?.available,
    ).toBe(false);
  });

  it("wraps only when repeat-all is enabled", () => {
    expect(nextQueueIndex(2, 3, 1, "off")).toBeNull();
    expect(nextQueueIndex(2, 3, 1, "all")).toBe(0);
    expect(nextQueueIndex(0, 3, -1, "all")).toBe(2);
  });
});
