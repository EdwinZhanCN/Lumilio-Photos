export type MediaPlaybackKind = "music" | "audio" | "video" | "live-photo";

type MediaPlaybackDetail = {
  kind: MediaPlaybackKind;
  active: boolean;
};

const mediaPlaybackEvent = "lumilio:media-playback";

export function announceMediaPlayback(kind: MediaPlaybackKind, active: boolean): void {
  if (typeof window === "undefined") return;
  window.dispatchEvent(
    new CustomEvent<MediaPlaybackDetail>(mediaPlaybackEvent, { detail: { kind, active } }),
  );
}

export function onMediaPlayback(listener: (detail: MediaPlaybackDetail) => void): () => void {
  if (typeof window === "undefined") return () => undefined;
  const handleEvent = (event: Event) => {
    const detail = (event as CustomEvent<MediaPlaybackDetail>).detail;
    if (detail?.kind && typeof detail.active === "boolean") listener(detail);
  };
  window.addEventListener(mediaPlaybackEvent, handleEvent);
  return () => window.removeEventListener(mediaPlaybackEvent, handleEvent);
}
