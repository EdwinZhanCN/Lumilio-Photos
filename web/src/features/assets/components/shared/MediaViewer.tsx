import React, { useRef, useEffect, useState } from "react";
import { Play, Pause, Volume2, VolumeX, Maximize, Music } from "lucide-react";
import { assetService } from "@/services/assetsService";

interface MediaViewerProps {
  asset: Asset;
  className?: string;
}

/**
 * Determines if an asset is a video based on MIME type or legacy type
 */
const isVideo = (asset: Asset): boolean => {
  if (asset.mime_type) {
    return asset.mime_type.startsWith("video/");
  }
  return asset.type === "VIDEO";
};

/**
 * Determines if an asset is audio based on MIME type or legacy type
 */
const isAudio = (asset: Asset): boolean => {
  if (asset.mime_type) {
    return asset.mime_type.startsWith("audio/");
  }
  return asset.type === "AUDIO";
};

/**
 * Formats time in seconds to MM:SS format
 */
const formatTime = (time: number): string => {
  const minutes = Math.floor(time / 60);
  const seconds = Math.floor(time % 60);
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
};

/**
 * MediaViewer component that renders appropriate viewer based on asset type
 */
const MediaViewer: React.FC<MediaViewerProps> = ({ asset, className = "" }) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const audioRef = useRef<HTMLAudioElement>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(1);
  const [isMuted, setIsMuted] = useState(false);

  const videoAsset = isVideo(asset);
  const audioAsset = isAudio(asset);
  const mediaRef = videoAsset ? videoRef : audioRef;

  // Get media source URL
  const mediaUrl = asset.asset_id 
    ? assetService.getOriginalFileUrl(asset.asset_id)
    : undefined;

  // For photos, get large thumbnail as fallback to original
  const imageUrl = !videoAsset && !audioAsset && asset.asset_id
    ? assetService.getThumbnailUrl(asset.asset_id, "large")
    : undefined;

  // Media event handlers
  useEffect(() => {
    const media = mediaRef.current;
    if (!media) return;

    const handleTimeUpdate = () => setCurrentTime(media.currentTime);
    const handleDurationChange = () => setDuration(media.duration);
    const handlePlay = () => setIsPlaying(true);
    const handlePause = () => setIsPlaying(false);
    const handleVolumeChange = () => {
      setVolume(media.volume);
      setIsMuted(media.muted);
    };

    media.addEventListener("timeupdate", handleTimeUpdate);
    media.addEventListener("durationchange", handleDurationChange);
    media.addEventListener("play", handlePlay);
    media.addEventListener("pause", handlePause);
    media.addEventListener("volumechange", handleVolumeChange);

    return () => {
      media.removeEventListener("timeupdate", handleTimeUpdate);
      media.removeEventListener("durationchange", handleDurationChange);
      media.removeEventListener("play", handlePlay);
      media.removeEventListener("pause", handlePause);
      media.removeEventListener("volumechange", handleVolumeChange);
    };
  }, [mediaRef]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!mediaRef.current) return;
      
      switch (e.code) {
        case "Space":
          e.preventDefault();
          if (isPlaying) {
            mediaRef.current.pause();
          } else {
            mediaRef.current.play();
          }
          break;
        case "ArrowLeft":
          e.preventDefault();
          mediaRef.current.currentTime = Math.max(0, mediaRef.current.currentTime - 10);
          break;
        case "ArrowRight":
          e.preventDefault();
          mediaRef.current.currentTime = Math.min(duration, mediaRef.current.currentTime + 10);
          break;
        case "ArrowUp":
          e.preventDefault();
          mediaRef.current.volume = Math.min(1, mediaRef.current.volume + 0.1);
          break;
        case "ArrowDown":
          e.preventDefault();
          mediaRef.current.volume = Math.max(0, mediaRef.current.volume - 0.1);
          break;
        case "KeyM":
          e.preventDefault();
          mediaRef.current.muted = !mediaRef.current.muted;
          break;
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isPlaying, duration, mediaRef]);

  const togglePlayPause = () => {
    if (!mediaRef.current) return;
    if (isPlaying) {
      mediaRef.current.pause();
    } else {
      mediaRef.current.play();
    }
  };

  const handleSeek = (e: React.ChangeEvent<HTMLInputElement>) => {
    const time = parseFloat(e.target.value);
    if (mediaRef.current) {
      mediaRef.current.currentTime = time;
      setCurrentTime(time);
    }
  };

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const vol = parseFloat(e.target.value);
    if (mediaRef.current) {
      mediaRef.current.volume = vol;
      setVolume(vol);
    }
  };

  const toggleMute = () => {
    if (mediaRef.current) {
      mediaRef.current.muted = !mediaRef.current.muted;
      setIsMuted(mediaRef.current.muted);
    }
  };

  const toggleFullscreen = () => {
    if (videoRef.current) {
      if (videoRef.current.requestFullscreen) {
        videoRef.current.requestFullscreen();
      }
    }
  };

  // Video player
  if (videoAsset && mediaUrl) {
    return (
      <div className={`relative w-full h-full flex items-center justify-center ${className}`}>
        <video
          ref={videoRef}
          src={mediaUrl}
          className="max-h-full max-w-full object-contain"
          controls={false}
          preload="metadata"
        />
        
        {/* Custom video controls overlay */}
        <div className="absolute bottom-4 left-4 right-4 bg-black/80 rounded-lg p-3 text-white">
          <div className="flex items-center gap-3 mb-2">
            <button
              onClick={togglePlayPause}
              className="btn btn-circle btn-sm bg-white/20 border-none hover:bg-white/30"
            >
              {isPlaying ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4 ml-0.5" />}
            </button>
            
            <div className="flex-1">
              <input
                type="range"
                min="0"
                max={duration || 0}
                value={currentTime}
                onChange={handleSeek}
                className="range range-sm range-primary w-full"
              />
            </div>
            
            <span className="text-sm font-mono">
              {formatTime(currentTime)} / {formatTime(duration)}
            </span>
          </div>
          
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <button
                onClick={toggleMute}
                className="btn btn-circle btn-sm bg-white/20 border-none hover:bg-white/30"
              >
                {isMuted ? <VolumeX className="w-4 h-4" /> : <Volume2 className="w-4 h-4" />}
              </button>
              
              <input
                type="range"
                min="0"
                max="1"
                step="0.1"
                value={isMuted ? 0 : volume}
                onChange={handleVolumeChange}
                className="range range-sm range-primary w-20"
              />
            </div>
            
            <button
              onClick={toggleFullscreen}
              className="btn btn-circle btn-sm bg-white/20 border-none hover:bg-white/30"
            >
              <Maximize className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    );
  }

  // Audio player
  if (audioAsset && mediaUrl) {
    return (
      <div className={`w-full h-full flex items-center justify-center ${className}`}>
        <div className="bg-gradient-to-br from-purple-600 to-pink-600 rounded-xl p-8 text-white shadow-2xl max-w-md w-full mx-4">
          <audio ref={audioRef} src={mediaUrl} preload="metadata" />
          
          {/* Audio visualization */}
          <div className="text-center mb-6">
            <div className="w-24 h-24 mx-auto mb-4 bg-white/20 rounded-full flex items-center justify-center">
              <Music className="w-12 h-12" />
            </div>
            <h3 className="text-xl font-bold mb-1">
              {asset.original_filename?.replace(/\.[^/.]+$/, "") || "Audio File"}
            </h3>
            <p className="text-white/70 text-sm">
              {asset.mime_type || "Audio"}
            </p>
          </div>
          
          {/* Progress bar */}
          <div className="mb-4">
            <input
              type="range"
              min="0"
              max={duration || 0}
              value={currentTime}
              onChange={handleSeek}
              className="range range-sm range-primary w-full"
            />
            <div className="flex justify-between text-sm text-white/70 mt-1">
              <span>{formatTime(currentTime)}</span>
              <span>{formatTime(duration)}</span>
            </div>
          </div>
          
          {/* Controls */}
          <div className="flex items-center justify-center gap-4">
            <button
              onClick={toggleMute}
              className="btn btn-circle btn-sm bg-white/20 border-none hover:bg-white/30"
            >
              {isMuted ? <VolumeX className="w-4 h-4" /> : <Volume2 className="w-4 h-4" />}
            </button>
            
            <button
              onClick={togglePlayPause}
              className="btn btn-circle btn-lg bg-white/30 border-none hover:bg-white/40"
            >
              {isPlaying ? <Pause className="w-6 h-6" /> : <Play className="w-6 h-6 ml-0.5" />}
            </button>
            
            <input
              type="range"
              min="0"
              max="1"
              step="0.1"
              value={isMuted ? 0 : volume}
              onChange={handleVolumeChange}
              className="range range-sm range-primary w-20"
            />
          </div>
          
          {/* Keyboard shortcuts hint */}
          <div className="text-center text-xs text-white/50 mt-4">
            Space: Play/Pause • ←→: Seek • ↑↓: Volume • M: Mute
          </div>
        </div>
      </div>
    );
  }

  // Photo display (fallback to existing behavior)
  if (imageUrl) {
    return (
      <div className={`h-screen w-screen flex items-center justify-center p-4 ${className}`}>
        <img
          src={imageUrl}
          alt={asset.original_filename || "Asset"}
          className="max-h-full max-w-full object-contain select-none"
        />
      </div>
    );
  }

  // Fallback for unsupported or missing media
  return (
    <div className={`w-full h-full flex items-center justify-center text-white ${className}`}>
      <div className="text-center">
        <div className="text-xl mb-2">Media not available</div>
        <div className="text-sm opacity-70">
          {asset.original_filename || "Unknown file"}
        </div>
        <div className="text-xs opacity-50 mt-1">
          {asset.mime_type || asset.type || "Unknown type"}
        </div>
      </div>
    </div>
  );
};

export default MediaViewer;