export type TimedLyric = { time: number; text: string };

/** LRC timestamps, repeated timestamps, and a global millisecond offset. */
export function parseTimedLyrics(content: string): TimedLyric[] {
  const offset = Number(content.match(/\[offset:([+-]?\d+)\]/i)?.[1] ?? 0) / 1000;
  const lines: TimedLyric[] = [];
  for (const line of content.split(/\r?\n/)) {
    const stamps = [...line.matchAll(/\[(\d+):(\d{2})(?:[.:](\d{1,3}))?\]/g)];
    const text = line.replace(/\[\d+:\d{2}(?:[.:]\d{1,3})?\]/g, "").trim();
    for (const stamp of stamps) {
      if (Number(stamp[2]) >= 60) continue;
      const time = Number(stamp[1]) * 60 + Number(stamp[2]) + Number(`0.${stamp[3] ?? 0}`) - offset;
      if (!Number.isFinite(time)) continue;
      lines.push({ time: Math.max(0, time), text });
    }
  }
  return lines.sort((a, b) => a.time - b.time);
}

export function activeLyricIndex(lines: TimedLyric[], time: number): number {
  let index = -1;
  for (let i = 0; i < lines.length && lines[i].time <= time; i++) index = i;
  return index;
}
