import { useRef, useState, useCallback } from "react";

/**
 * Encapsulates the play/stop lifecycle for a Live Photo video element.
 *
 * Attach `videoRef` to a `<video>` element whose visibility is driven by
 * `isPlaying`. Call `handlePlay` on pointer-enter and `handleStop` on
 * pointer-leave / pointer-up to get a native-feeling Live Photo experience.
 *
 * The video is rewound to the start before each play so every hover
 * shows the full motion clip.  After `handleStop` the video is paused
 * only after the CSS fade-out transition finishes (300 ms) so the
 * transition completes without a jarring cut.
 */
export function useLivePhotoPlayback() {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const stopTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const cancelPendingStop = () => {
    if (stopTimerRef.current !== null) {
      clearTimeout(stopTimerRef.current);
      stopTimerRef.current = null;
    }
  };

  const handlePlay = useCallback(() => {
    cancelPendingStop();
    const video = videoRef.current;
    if (!video) return;
    video.currentTime = 0;
    video
      .play()
      .then(() => setIsPlaying(true))
      .catch(() => {
        // Autoplay blocked or element unmounted — silently ignore.
      });
  }, []);

  const handleStop = useCallback(() => {
    setIsPlaying(false);
    // Wait for the CSS opacity transition (300 ms) before pausing so the
    // fade-out completes without a jump back to frame 0.
    stopTimerRef.current = setTimeout(() => {
      const video = videoRef.current;
      if (!video) return;
      video.pause();
      video.currentTime = 0;
      stopTimerRef.current = null;
    }, 300);
  }, []);

  /** Call when the video's `onEnded` fires to gracefully return to still. */
  const handleEnded = useCallback(() => {
    setIsPlaying(false);
    stopTimerRef.current = setTimeout(() => {
      const video = videoRef.current;
      if (!video) return;
      video.currentTime = 0;
      stopTimerRef.current = null;
    }, 300);
  }, []);

  return { videoRef, isPlaying, handlePlay, handleStop, handleEnded };
}
