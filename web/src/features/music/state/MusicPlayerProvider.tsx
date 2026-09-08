import { readPlayback, serializePlayback } from "../model/playbackStorage";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import type { components } from "@/lib/http-commons";
import { client } from "@/lib/http-commons/queryClient";
import { assetUrls } from "@/lib/assets/assetUrls";
import { announceMediaPlayback, onMediaPlayback } from "@/lib/media/mediaCoordinator";
import {
  nextQueueIndex,
  playbackEntryToQueueItem,
  trackId,
  type MusicPlaybackEntry,
  type MusicPlaybackSource,
  type MusicQueueItem,
  type MusicTrack,
} from "../model/music";

type RepeatMode = "off" | "all" | "one";
type PlaybackPage = components["schemas"]["dto.MusicPlaybackPageDTO"];

type PlayerState = {
  queue: MusicQueueItem[];
  currentIndex: number;
  sessionId?: string;
  totalEntries: number;
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  repeat: RepeatMode;
  shuffle: boolean;
  isLoading: boolean;
  error?: string;
};

type MusicPlayerContextValue = {
  current: MusicQueueItem | null;
  queue: MusicQueueItem[];
  currentIndex: number;
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  repeat: RepeatMode;
  shuffle: boolean;
  isLoading: boolean;
  error?: string;
  playTrack: (track: MusicTrack, source?: MusicPlaybackSource, entryId?: string) => Promise<void>;
  playSource: (
    source: MusicPlaybackSource,
    selectedTrack?: MusicTrack,
    entryId?: string,
  ) => Promise<void>;
  toggle: () => void;
  next: () => void;
  previous: () => void;
  seek: (seconds: number) => void;
  setVolume: (volume: number) => void;
  toggleShuffle: () => void;
  cycleRepeat: () => void;
  clear: () => void;
  playQueueIndex: (index: number) => void;
  removeQueueIndex: (index: number) => void;
  enqueue: (track: MusicTrack) => void;
};

const MusicPlayerContext = createContext<MusicPlayerContextValue | null>(null);

function queueFromPage(items: MusicPlaybackEntry[] | undefined): MusicQueueItem[] {
  return (items ?? [])
    .map(playbackEntryToQueueItem)
    .filter((item): item is MusicQueueItem => item !== null);
}

async function fetchPlaybackPage(sessionId: string, offset: number): Promise<PlaybackPage> {
  const { data, error } = await client.GET("/api/v1/music/playback-sessions/{id}/entries", {
    params: { path: { id: sessionId }, query: { limit: 200, offset } },
  });
  if (error || !data) {
    throw new Error("The music queue could not be loaded.");
  }
  return data;
}

export function MusicPlayerProvider({
  children,
  ownerId,
}: PropsWithChildren<{ ownerId?: number }>) {
  const storageKey = ownerId == null ? undefined : `lumilio:music:playback:v1:${ownerId}`;
  const [restored] = useState(() => {
    try {
      return storageKey ? readPlayback(localStorage.getItem(storageKey)) : null;
    } catch {
      return null;
    }
  });
  const [state, setState] = useState<PlayerState>({
    queue: [],
    currentIndex: 0,
    totalEntries: 0,
    isPlaying: false,
    currentTime: 0,
    duration: 0,
    volume: 0.8,
    repeat: "off",
    shuffle: false,
    isLoading: false,
    ...restored,
  });
  const stateRef = useRef(state);
  const audioRef = useRef<HTMLAudioElement>(null);
  const shouldPlayRef = useRef(false);
  const requestRef = useRef(0);
  const restoreTimeRef = useRef(restored?.currentTime ?? 0);

  useEffect(() => {
    if (!storageKey) return;
    const save = () => {
      try {
        localStorage.setItem(storageKey, serializePlayback(stateRef.current));
      } catch {
        /* Storage may be unavailable or full. */
      }
    };
    const timer = window.setInterval(save, 3000);
    window.addEventListener("pagehide", save);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("pagehide", save);
      save();
    };
  }, [storageKey]);

  useEffect(() => {
    stateRef.current = state;
  }, [state]);

  const current = state.queue[state.currentIndex] ?? null;
  const currentTrackId = current ? trackId(current.track) : "";

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    audio.volume = state.volume;
  }, [state.volume]);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    if (!currentTrackId || !current?.available) {
      audio.pause();
      audio.removeAttribute("src");
      audio.load();
      setState((previous) => ({ ...previous, isPlaying: false, currentTime: 0, duration: 0 }));
      return;
    }

    audio.src = assetUrls.getWebAudioUrl(currentTrackId);
    audio.load();
    setState((previous) => ({
      ...previous,
      currentTime: 0,
      duration: current.track.duration ?? 0,
      error: undefined,
    }));
    if (shouldPlayRef.current) {
      void audio.play().catch(() => {
        setState((previous) => ({
          ...previous,
          isPlaying: false,
          error: "Playback needs a user gesture or the file is unavailable.",
        }));
      });
    }
  }, [current, currentTrackId]);

  const replaceCurrentWithSelectedTrack = useCallback(
    (items: MusicQueueItem[], selectedTrack?: MusicTrack): MusicQueueItem[] => {
      if (!selectedTrack || !trackId(selectedTrack)) return items;
      const selectedID = trackId(selectedTrack);
      const existingIndex = items.findIndex((item) => trackId(item.track) === selectedID);
      const selectedItem: MusicQueueItem = { track: selectedTrack, available: true };
      if (existingIndex >= 0) {
        return items.map((item, index) =>
          index === existingIndex ? { ...item, track: selectedTrack } : item,
        );
      }
      return [selectedItem, ...items];
    },
    [],
  );

  const playSource = useCallback(
    async (source: MusicPlaybackSource, selectedTrack?: MusicTrack, entryId?: string) => {
      if (!source.kind) return;
      restoreTimeRef.current = 0;
      const request = ++requestRef.current;
      setState((previous) => ({ ...previous, isLoading: true, error: undefined }));
      try {
        const { data, error } = await client.POST("/api/v1/music/playback-sessions", {
          body: source,
        });
        if (error || !data?.session_id) {
          throw new Error("The music queue could not be created.");
        }
        const page = await fetchPlaybackPage(data.session_id, 0);
        const items = [...(page.items ?? [])];
        const total = data.total_entries ?? page.total ?? items.length;
        while (items.length < total) {
          const next = await fetchPlaybackPage(data.session_id, items.length);
          if (!next.items?.length) throw new Error("The music queue is incomplete. Try again.");
          items.push(...next.items);
          if (request !== requestRef.current) return;
        }
        if (request !== requestRef.current) return;
        const queue = replaceCurrentWithSelectedTrack(queueFromPage(items), selectedTrack);
        const selectedID = selectedTrack ? trackId(selectedTrack) : "";
        const selectedIndex = selectedID
          ? Math.max(
              0,
              queue.findIndex((item) =>
                entryId ? item.entryId === entryId : trackId(item.track) === selectedID,
              ),
            )
          : 0;
        shouldPlayRef.current = true;
        setState((previous) => ({
          ...previous,
          queue,
          currentIndex: selectedIndex,
          sessionId: data.session_id,
          totalEntries: queue.length,
          isLoading: false,
          isPlaying: true,
          currentTime: 0,
          duration: queue[selectedIndex]?.track.duration ?? 0,
          error: undefined,
        }));
      } catch (error) {
        if (request !== requestRef.current) return;
        setState((previous) => ({
          ...previous,
          isLoading: false,
          isPlaying: false,
          error: error instanceof Error ? error.message : "The music queue could not be loaded.",
        }));
      }
    },
    [replaceCurrentWithSelectedTrack],
  );

  const playTrack = useCallback(
    async (track: MusicTrack, source?: MusicPlaybackSource, entryId?: string) => {
      if (!trackId(track)) return;
      if (source) {
        await playSource(source, track, entryId);
        return;
      }
      restoreTimeRef.current = 0;
      requestRef.current += 1;
      shouldPlayRef.current = true;
      setState((previous) => ({
        ...previous,
        isLoading: false,
        queue: [{ track, available: true }],
        currentIndex: 0,
        sessionId: undefined,
        totalEntries: 1,
        isPlaying: true,
        currentTime: 0,
        duration: track.duration ?? 0,
        error: undefined,
      }));
    },
    [playSource],
  );

  const move = useCallback((direction: 1 | -1) => {
    const snapshot = stateRef.current;
    if (snapshot.queue.length === 0) return;
    if (snapshot.shuffle) {
      const available = snapshot.queue
        .map((item, index) => (item.available && index !== snapshot.currentIndex ? index : -1))
        .filter((index) => index >= 0);
      if (available.length > 0) {
        const nextIndex = available[Math.floor(Math.random() * available.length)] ?? null;
        if (nextIndex != null) {
          shouldPlayRef.current = true;
          setState((previous) => ({ ...previous, currentIndex: nextIndex, isPlaying: true }));
          return;
        }
      }
    }

    let nextIndex = nextQueueIndex(
      snapshot.currentIndex,
      snapshot.queue.length,
      direction,
      snapshot.repeat === "all" ? "all" : "off",
    );
    let inspected = 0;
    while (nextIndex != null && inspected < snapshot.queue.length) {
      if (snapshot.queue[nextIndex]?.available) break;
      nextIndex = nextQueueIndex(nextIndex, snapshot.queue.length, direction, "off");
      inspected += 1;
    }
    if (nextIndex != null && snapshot.queue[nextIndex]?.available) {
      shouldPlayRef.current = true;
      if (nextIndex === snapshot.currentIndex && audioRef.current) {
        audioRef.current.currentTime = 0;
        void audioRef.current.play().catch(() => undefined);
      }
      setState((previous) => ({ ...previous, currentIndex: nextIndex!, isPlaying: true }));
      return;
    }

    shouldPlayRef.current = false;
    audioRef.current?.pause();
    setState((previous) => ({ ...previous, isPlaying: false }));
  }, []);

  const toggle = useCallback(() => {
    const audio = audioRef.current;
    if (!audio || !currentTrackId || !current?.available) return;
    if (audio.paused) {
      shouldPlayRef.current = true;
      void audio.play().catch(() => undefined);
    } else {
      shouldPlayRef.current = false;
      audio.pause();
    }
  }, [current, currentTrackId]);

  const seek = useCallback((seconds: number) => {
    const audio = audioRef.current;
    if (!audio || !Number.isFinite(seconds)) return;
    audio.currentTime = Math.max(0, seconds);
    setState((previous) => ({ ...previous, currentTime: audio.currentTime }));
  }, []);

  const setVolume = useCallback((volume: number) => {
    const nextVolume = Math.min(1, Math.max(0, volume));
    setState((previous) => ({ ...previous, volume: nextVolume }));
  }, []);

  const clear = useCallback(() => {
    restoreTimeRef.current = 0;
    requestRef.current += 1;
    const sessionId = stateRef.current.sessionId;
    if (sessionId) {
      void client.DELETE("/api/v1/music/playback-sessions/{id}", {
        params: { path: { id: sessionId } },
      });
    }
    shouldPlayRef.current = false;
    audioRef.current?.pause();
    setState((previous) => ({
      ...previous,
      queue: [],
      isLoading: false,
      currentIndex: 0,
      sessionId: undefined,
      totalEntries: 0,
      isPlaying: false,
      currentTime: 0,
      duration: 0,
      error: undefined,
    }));
  }, []);

  const cycleRepeat = useCallback(() => {
    setState((previous) => ({
      ...previous,
      repeat: previous.repeat === "off" ? "all" : previous.repeat === "all" ? "one" : "off",
    }));
  }, []);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (
        event.defaultPrevented ||
        target?.closest("button, a, [role=button]") ||
        target?.isContentEditable ||
        target?.tagName === "INPUT" ||
        target?.tagName === "TEXTAREA" ||
        target?.tagName === "SELECT"
      ) {
        return;
      }
      if (event.code === "Space") {
        event.preventDefault();
        toggle();
      } else if (event.key === "ArrowRight" && event.shiftKey) {
        event.preventDefault();
        move(1);
      } else if (event.key === "ArrowLeft" && event.shiftKey) {
        event.preventDefault();
        move(-1);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [move, toggle]);

  useEffect(() => {
    return onMediaPlayback(({ kind, active }) => {
      if (kind === "music" || !active) return;
      shouldPlayRef.current = false;
      audioRef.current?.pause();
    });
  }, []);

  const value = useMemo<MusicPlayerContextValue>(
    () => ({
      current,
      queue: state.queue,
      currentIndex: state.currentIndex,
      isPlaying: state.isPlaying,
      currentTime: state.currentTime,
      duration: state.duration,
      volume: state.volume,
      repeat: state.repeat,
      shuffle: state.shuffle,
      isLoading: state.isLoading,
      error: state.error,
      playTrack,
      playSource,
      toggle,
      next: () => move(1),
      previous: () => move(-1),
      seek,
      setVolume,
      toggleShuffle: () => setState((previous) => ({ ...previous, shuffle: !previous.shuffle })),
      cycleRepeat,
      clear,
      playQueueIndex: (index) => {
        if (!state.queue[index]?.available) return;
        shouldPlayRef.current = true;
        if (index === state.currentIndex) {
          void audioRef.current?.play().catch(() => undefined);
        } else {
          setState((previous) => ({ ...previous, currentIndex: index, isPlaying: true }));
        }
      },
      removeQueueIndex: (index) => {
        setState((previous) => {
          if (!previous.queue[index]) return previous;
          const queue = previous.queue.filter((_, position) => position !== index);
          const currentIndex = Math.max(
            0,
            Math.min(
              queue.length - 1,
              previous.currentIndex - (index < previous.currentIndex ? 1 : 0),
            ),
          );
          return { ...previous, queue, currentIndex, totalEntries: queue.length };
        });
      },
      enqueue: (track) => {
        if (!trackId(track)) return;
        setState((previous) => ({
          ...previous,
          queue: [...previous.queue, { track, available: true }],
          totalEntries: previous.queue.length + 1,
        }));
      },
    }),
    [clear, current, cycleRepeat, move, playSource, playTrack, seek, setVolume, state, toggle],
  );

  return (
    <MusicPlayerContext.Provider value={value}>
      <audio
        ref={audioRef}
        preload="metadata"
        aria-hidden="true"
        onTimeUpdate={(event) => {
          const currentTime = event.currentTarget.currentTime;
          setState((previous) => ({ ...previous, currentTime }));
        }}
        onLoadedMetadata={(event) => {
          const duration = event.currentTarget.duration;
          if (restoreTimeRef.current > 0 && Number.isFinite(duration)) {
            event.currentTarget.currentTime = Math.min(restoreTimeRef.current, duration);
            restoreTimeRef.current = 0;
          }
          setState((previous) => ({
            ...previous,
            duration: Number.isFinite(duration) ? duration : previous.duration,
          }));
        }}
        onPlay={() => {
          announceMediaPlayback("music", true);
          setState((previous) => ({ ...previous, isPlaying: true }));
        }}
        onPause={() => {
          announceMediaPlayback("music", false);
          setState((previous) => ({ ...previous, isPlaying: false }));
        }}
        onEnded={() => {
          if (stateRef.current.repeat === "one") {
            const audio = audioRef.current;
            if (audio) {
              audio.currentTime = 0;
              void audio.play().catch(() => undefined);
            }
            return;
          }
          move(1);
        }}
        onError={() =>
          setState((previous) => ({
            ...previous,
            isPlaying: false,
            error: "This audio file is unavailable.",
          }))
        }
      />
      {children}
    </MusicPlayerContext.Provider>
  );
}

export function useMusicPlayer(): MusicPlayerContextValue {
  const value = useContext(MusicPlayerContext);
  if (!value) {
    throw new Error("useMusicPlayer must be used inside MusicPlayerProvider");
  }
  return value;
}
