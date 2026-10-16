import type { MusicQueueItem } from "./music";

export type SavedPlayback = {
  queue: MusicQueueItem[];
  currentIndex: number;
  currentTime: number;
  volume: number;
  repeat: "off" | "all" | "one";
  shuffle: boolean;
};

export function readPlayback(raw: string | null): SavedPlayback | null {
  try {
    const data = JSON.parse(raw ?? "null");
    if (!data || data.version !== 1 || !Array.isArray(data.queue) || data.queue.length > 10000)
      return null;
    const queue: MusicQueueItem[] = data.queue.map((item: unknown) => {
      if (!item || typeof item !== "object" || !("id" in item) || typeof item.id !== "string")
        throw new Error("Invalid queue");
      return {
        track: {
          track_id: item.id,
          title: "title" in item && typeof item.title === "string" ? item.title : undefined,
        },
        available: Boolean(item.id),
      };
    });
    if (
      !Number.isInteger(data.currentIndex) ||
      data.currentIndex < 0 ||
      data.currentIndex >= Math.max(1, queue.length)
    )
      return null;
    return {
      queue,
      currentIndex: data.currentIndex,
      currentTime: Number.isFinite(data.currentTime) ? Math.max(0, data.currentTime) : 0,
      volume: Number.isFinite(data.volume) ? Math.min(1, Math.max(0, data.volume)) : 0.8,
      repeat: data.repeat === "all" || data.repeat === "one" ? data.repeat : "off",
      shuffle: data.shuffle === true,
    };
  } catch {
    return null;
  }
}

export function serializePlayback(state: SavedPlayback): string {
  return JSON.stringify({
    currentIndex: state.currentIndex,
    currentTime: state.currentTime,
    volume: state.volume,
    repeat: state.repeat,
    shuffle: state.shuffle,
    version: 1,
    queue: state.queue.map((item) => ({
      id: item.track.track_id ?? "",
      title: item.track.title ?? item.savedTitle,
    })),
  });
}
