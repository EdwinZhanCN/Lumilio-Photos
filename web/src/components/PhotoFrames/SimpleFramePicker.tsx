import React, {
  useState,
  useCallback,
  useMemo,
  useRef,
  useEffect,
} from "react";
import { getAllFrames } from "./frameRegistry";
import { FrameDefinition } from "./types";
import {
  PhotoIcon,
  XMarkIcon,
  ArrowDownTrayIcon,
} from "@heroicons/react/24/outline";

interface SimpleFramePickerProps {
  imageUrl: string | null;
  metadata?: string;
  onFramedImageChange?: (framedImageUrl: string | null) => void;
  onExport?: (dataUrl: string, filename: string) => void;
}

export const SimpleFramePicker: React.FC<SimpleFramePickerProps> = ({
  imageUrl,
  metadata,
  onFramedImageChange,
  onExport,
}) => {
  const [selectedFrameId, setSelectedFrameId] = useState<string | null>(null);
  const [isGeneratingFrame, setIsGeneratingFrame] = useState(false);
  const [framedImageUrl, setFramedImageUrl] = useState<string | null>(null);

  const canvasRef = useRef<HTMLCanvasElement>(null);
  const allFrames = useMemo(() => getAllFrames(), []);

  const selectedFrame = useMemo(() => {
    return selectedFrameId
      ? allFrames.find((f) => f.id === selectedFrameId)
      : null;
  }, [selectedFrameId, allFrames]);

  // Generate framed image when frame is selected
  const generateFramedImage = useCallback(async () => {
    if (!selectedFrame || !imageUrl || !canvasRef.current) {
      setFramedImageUrl(null);
      onFramedImageChange?.(null);
      return;
    }

    setIsGeneratingFrame(true);

    try {
      const canvas = canvasRef.current;
      const ctx = canvas.getContext("2d");
      if (!ctx) return;

      // Load the image
      const img = new Image();
      img.crossOrigin = "anonymous";

      await new Promise<void>((resolve, reject) => {
        img.onload = () => resolve();
        img.onerror = reject;
        img.src = imageUrl;
      });

      // Apply frame with default settings
      switch (selectedFrame.id) {
        case "glass-frame":
          await drawGlassFrame(ctx, img);
          break;
        case "classic-frame":
          await drawClassicFrame(ctx, img);
          break;
        case "polaroid-frame":
          await drawPolaroidFrame(ctx, img);
          break;
        default:
          await drawSimpleFrame(ctx, img);
      }

      // Convert canvas to blob URL
      canvas.toBlob((blob) => {
        if (blob) {
          const url = URL.createObjectURL(blob);
          setFramedImageUrl(url);
          onFramedImageChange?.(url);
        }
      }, "image/png");
    } catch (error) {
      console.error("Failed to generate framed image:", error);
    } finally {
      setIsGeneratingFrame(false);
    }
  }, [selectedFrame, imageUrl, metadata, onFramedImageChange]);

  // Simple frame drawing functions with fixed, good-looking defaults
  const drawSimpleFrame = async (
    ctx: CanvasRenderingContext2D,
    img: HTMLImageElement,
  ) => {
    const borderWidth = 20;
    ctx.canvas.width = img.width + borderWidth * 2;
    ctx.canvas.height = img.height + borderWidth * 2;

    ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
    ctx.fillStyle = "#8B4513";
    ctx.fillRect(0, 0, ctx.canvas.width, ctx.canvas.height);
    ctx.drawImage(img, borderWidth, borderWidth);
  };

  const drawGlassFrame = async (
    ctx: CanvasRenderingContext2D,
    img: HTMLImageElement,
  ) => {
    const borderWidth = 80;
    const cornerRadius = 25;
    const metadataHeight = metadata ? 70 : 0;
    const vignetteWidth = 60;
    const imageScale = 0.75; // Original image will be 75% of available space

    ctx.canvas.width = img.width + borderWidth * 2;
    ctx.canvas.height = img.height + borderWidth * 2 + metadataHeight;

    ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);

    // Helper function for rounded rectangles
    const drawRoundedRect = (
      x: number,
      y: number,
      width: number,
      height: number,
      radius: number,
    ) => {
      ctx.beginPath();
      ctx.moveTo(x + radius, y);
      ctx.lineTo(x + width - radius, y);
      ctx.quadraticCurveTo(x + width, y, x + width, y + radius);
      ctx.lineTo(x + width, y + height - radius);
      ctx.quadraticCurveTo(
        x + width,
        y + height,
        x + width - radius,
        y + height,
      );
      ctx.lineTo(x + radius, y + height);
      ctx.quadraticCurveTo(x, y + height, x, y + height - radius);
      ctx.lineTo(x, y + radius);
      ctx.quadraticCurveTo(x, y, x + radius, y);
      ctx.closePath();
    };

    // Create sophisticated background with depth
    ctx.save();
    ctx.shadowColor = "rgba(0, 0, 0, 0.25)";
    ctx.shadowBlur = 40;
    ctx.shadowOffsetX = 8;
    ctx.shadowOffsetY = 12;

    drawRoundedRect(0, 0, ctx.canvas.width, ctx.canvas.height, cornerRadius);

    // Multi-layered glass effect
    const glassGradient = ctx.createRadialGradient(
      ctx.canvas.width * 0.3,
      ctx.canvas.height * 0.2,
      0,
      ctx.canvas.width / 2,
      ctx.canvas.height / 2,
      Math.max(ctx.canvas.width, ctx.canvas.height) * 0.8,
    );
    glassGradient.addColorStop(0, "rgba(255, 255, 255, 0.6)");
    glassGradient.addColorStop(0.3, "rgba(255, 255, 255, 0.2)");
    glassGradient.addColorStop(0.7, "rgba(240, 240, 255, 0.15)");
    glassGradient.addColorStop(1, "rgba(220, 230, 255, 0.3)");
    ctx.fillStyle = glassGradient;
    ctx.fill();
    ctx.restore();

    // Add subtle film grain texture
    ctx.save();
    drawRoundedRect(0, 0, ctx.canvas.width, ctx.canvas.height, cornerRadius);
    ctx.clip();

    for (let i = 0; i < 200; i++) {
      const x = Math.random() * ctx.canvas.width;
      const y = Math.random() * ctx.canvas.height;
      const opacity = Math.random() * 0.02;
      ctx.fillStyle = `rgba(255, 255, 255, ${opacity})`;
      ctx.fillRect(x, y, 1, 1);
    }
    ctx.restore();

    // Create blurred background image first
    ctx.save();

    // Draw larger blurred background image without clipping for glass edge
    ctx.filter = "blur(60px)";
    const blurScale = 1.2; // Make blur image 20% larger
    const blurOffsetX = borderWidth - (img.width * (blurScale - 1)) / 2;
    const blurOffsetY = borderWidth - (img.height * (blurScale - 1)) / 2;

    ctx.drawImage(
      img,
      blurOffsetX,
      blurOffsetY,
      img.width * blurScale,
      img.height * blurScale,
    );

    ctx.restore();

    // Draw shadow for the sharp original image first
    ctx.save();
    ctx.filter = "none"; // Reset filter

    const scaledWidth = img.width * imageScale;
    const scaledHeight = img.height * imageScale;
    const centerX = borderWidth + (img.width - scaledWidth) / 2;
    const centerY = borderWidth + (img.height - scaledHeight) / 2;

    const smallImageRadius = 60;

    // Draw shadow shape
    ctx.shadowColor = "rgba(0, 0, 0, 0.4)";
    ctx.shadowBlur = 60;
    ctx.shadowOffsetX = 15;
    ctx.shadowOffsetY = 15;
    ctx.fillStyle = "rgba(255, 255, 255, 1)"; // Temporary fill for shadow
    drawRoundedRect(
      centerX,
      centerY,
      scaledWidth,
      scaledHeight,
      smallImageRadius,
    );
    ctx.fill();

    // Reset shadow and draw the actual image
    ctx.shadowColor = "transparent";
    ctx.shadowBlur = 0;
    ctx.shadowOffsetX = 0;
    ctx.shadowOffsetY = 0;

    drawRoundedRect(
      centerX,
      centerY,
      scaledWidth,
      scaledHeight,
      smallImageRadius,
    );
    ctx.clip();

    // Draw the sharp original image
    ctx.drawImage(img, centerX, centerY, scaledWidth, scaledHeight);

    ctx.restore();

    // Enhanced glass reflection with multiple layers
    ctx.save();
    drawRoundedRect(0, 0, ctx.canvas.width, ctx.canvas.height, cornerRadius);
    ctx.clip();

    // Primary reflection
    const primaryReflection = ctx.createLinearGradient(
      0,
      0,
      ctx.canvas.width * 0.7,
      ctx.canvas.height * 0.7,
    );
    primaryReflection.addColorStop(0, "rgba(255, 255, 255, 0.2)");
    primaryReflection.addColorStop(0.4, "rgba(255, 255, 255, 0.05)");
    primaryReflection.addColorStop(1, "rgba(255, 255, 255, 0)");
    ctx.fillStyle = primaryReflection;
    ctx.fill();

    // Secondary highlight
    const highlight = ctx.createLinearGradient(
      0,
      0,
      ctx.canvas.width * 0.3,
      ctx.canvas.height * 0.3,
    );
    highlight.addColorStop(0, "rgba(255, 255, 255, 0.15)");
    highlight.addColorStop(0.5, "rgba(255, 255, 255, 0.02)");
    highlight.addColorStop(1, "rgba(255, 255, 255, 0)");
    ctx.fillStyle = highlight;
    ctx.fill();

    ctx.restore();

    // Draw enhanced metadata if provided
    if (metadata) {
      const metaY = borderWidth + img.height;
      ctx.save();
      ctx.beginPath();
      ctx.rect(borderWidth, metaY, img.width, metadataHeight);
      ctx.clip();

      // Sophisticated metadata background
      const metaGradient = ctx.createLinearGradient(
        0,
        metaY,
        0,
        metaY + metadataHeight,
      );
      metaGradient.addColorStop(0, "rgba(20, 30, 40, 0.85)");
      metaGradient.addColorStop(0.5, "rgba(10, 20, 30, 0.9)");
      metaGradient.addColorStop(1, "rgba(5, 15, 25, 0.95)");
      ctx.fillStyle = metaGradient;
      ctx.fillRect(borderWidth, metaY, img.width, metadataHeight);

      // Add subtle texture to metadata area
      for (let i = 0; i < 50; i++) {
        const x = borderWidth + Math.random() * img.width;
        const y = metaY + Math.random() * metadataHeight;
        const opacity = Math.random() * 0.05;
        ctx.fillStyle = `rgba(255, 255, 255, ${opacity})`;
        ctx.fillRect(x, y, 1, 1);
      }

      // Enhanced text styling
      ctx.fillStyle = "rgba(255, 255, 255, 0.95)";
      ctx.font =
        '16px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';
      ctx.textAlign = "left";
      ctx.textBaseline = "middle";

      const textX = borderWidth + 20;
      const textY = metaY + metadataHeight / 2;
      const maxWidth = img.width - 40;

      // Add text shadow for better readability
      ctx.save();
      ctx.shadowColor = "rgba(0, 0, 0, 0.8)";
      ctx.shadowBlur = 4;
      ctx.shadowOffsetX = 1;
      ctx.shadowOffsetY = 1;

      let displayText = metadata;
      if (ctx.measureText(displayText).width > maxWidth) {
        while (
          ctx.measureText(displayText + "...").width > maxWidth &&
          displayText.length > 0
        ) {
          displayText = displayText.slice(0, -1);
        }
        displayText += "...";
      }

      ctx.fillText(displayText, textX, textY);
      ctx.restore();
      ctx.restore();
    }
  };

  const drawClassicFrame = async (
    ctx: CanvasRenderingContext2D,
    img: HTMLImageElement,
  ) => {
    const borderWidth = 30;
    const matteWidth = 20;

    ctx.canvas.width = img.width + (borderWidth + matteWidth) * 2;
    ctx.canvas.height = img.height + (borderWidth + matteWidth) * 2;

    ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);

    // Draw outer frame
    ctx.fillStyle = "#8B4513";
    ctx.fillRect(0, 0, ctx.canvas.width, ctx.canvas.height);

    // Draw matte
    ctx.fillStyle = "#F5F5DC";
    ctx.fillRect(
      borderWidth,
      borderWidth,
      ctx.canvas.width - borderWidth * 2,
      ctx.canvas.height - borderWidth * 2,
    );

    // Draw inner border
    ctx.fillStyle = "#DAA520";
    const innerX = borderWidth + matteWidth;
    const innerY = borderWidth + matteWidth;
    const innerWidth = ctx.canvas.width - (borderWidth + matteWidth) * 2;
    const innerHeight = ctx.canvas.height - (borderWidth + matteWidth) * 2;
    ctx.fillRect(innerX, innerY, innerWidth, innerHeight);

    // Draw image
    ctx.drawImage(
      img,
      borderWidth + matteWidth + 5,
      borderWidth + matteWidth + 5,
      img.width - 10,
      img.height - 10,
    );

    // Add wood grain texture
    for (let i = 0; i < borderWidth; i += 2) {
      const alpha = 0.1 * Math.sin(i * 0.1);
      ctx.fillStyle = `rgba(0, 0, 0, ${Math.abs(alpha)})`;
      ctx.fillRect(0, i, ctx.canvas.width, 1);
      ctx.fillRect(0, ctx.canvas.height - i - 1, ctx.canvas.width, 1);
      ctx.fillRect(i, 0, 1, ctx.canvas.height);
      ctx.fillRect(ctx.canvas.width - i - 1, 0, 1, ctx.canvas.height);
    }
  };

  const drawPolaroidFrame = async (
    ctx: CanvasRenderingContext2D,
    img: HTMLImageElement,
  ) => {
    const borderWidth = 20;
    const bottomHeight = 80;
    const imageSize = Math.min(img.width, img.height);
    const totalWidth = imageSize + borderWidth * 2;
    const totalHeight = imageSize + borderWidth + bottomHeight;

    ctx.canvas.width = totalWidth;
    ctx.canvas.height = totalHeight;

    ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);

    // Draw background
    ctx.fillStyle = "#FEFEFE";
    ctx.fillRect(0, 0, totalWidth, totalHeight);

    // Add vintage effect
    const vintageGradient = ctx.createRadialGradient(
      totalWidth / 2,
      totalHeight / 2,
      0,
      totalWidth / 2,
      totalHeight / 2,
      Math.max(totalWidth, totalHeight),
    );
    vintageGradient.addColorStop(0, "rgba(255, 248, 220, 0.1)");
    vintageGradient.addColorStop(1, "rgba(222, 184, 135, 0.2)");
    ctx.fillStyle = vintageGradient;
    ctx.fillRect(0, 0, totalWidth, totalHeight);

    // Draw image (cropped to square)
    const sourceX = (img.width - imageSize) / 2;
    const sourceY = (img.height - imageSize) / 2;
    ctx.drawImage(
      img,
      sourceX,
      sourceY,
      imageSize,
      imageSize,
      borderWidth,
      borderWidth,
      imageSize,
      imageSize,
    );

    // Draw photo edge
    ctx.strokeStyle = "rgba(0, 0, 0, 0.1)";
    ctx.lineWidth = 1;
    ctx.strokeRect(borderWidth, borderWidth, imageSize, imageSize);

    // Draw metadata if provided
    if (metadata) {
      ctx.fillStyle = "#2D3748";
      ctx.font = '16px "Comic Sans MS", cursive';
      ctx.textAlign = "center";
      ctx.fillText(metadata, totalWidth / 2, imageSize + borderWidth + 40);
    }
  };

  useEffect(() => {
    if (selectedFrame && imageUrl) {
      generateFramedImage();
    }
  }, [generateFramedImage]);

  const handleFrameSelect = useCallback(
    (frame: FrameDefinition) => {
      if (selectedFrameId === frame.id) {
        // Deselect if clicking the same frame
        setSelectedFrameId(null);
        setFramedImageUrl(null);
        onFramedImageChange?.(null);
      } else {
        setSelectedFrameId(frame.id);
      }
    },
    [selectedFrameId, onFramedImageChange],
  );

  const handleExport = useCallback(() => {
    if (framedImageUrl && onExport) {
      fetch(framedImageUrl)
        .then((res) => res.blob())
        .then((blob) => {
          const reader = new FileReader();
          reader.onload = () => {
            const dataUrl = reader.result as string;
            const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
            const frameName =
              selectedFrame?.name.toLowerCase().replace(/\s+/g, "-") || "frame";
            const filename = `${frameName}-${timestamp}.png`;
            onExport(dataUrl, filename);
          };
          reader.readAsDataURL(blob);
        });
    }
  }, [framedImageUrl, onExport, selectedFrame]);

  if (!imageUrl) {
    return (
      <div className="text-center p-6">
        <PhotoIcon className="w-12 h-12 mx-auto text-base-content/30 mb-2" />
        <p className="text-sm text-base-content/70">
          Select an image to add frames
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Hidden canvas for frame generation */}
      <canvas ref={canvasRef} style={{ display: "none" }} />

      {/* Selected Frame Info */}
      {selectedFrame && (
        <div className="bg-primary/10 border border-primary/20 p-3 rounded-lg">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <span className="font-medium text-sm">{selectedFrame.name}</span>
              {isGeneratingFrame && (
                <span className="loading loading-spinner loading-xs"></span>
              )}
            </div>
            <div className="flex items-center space-x-1">
              <button
                onClick={handleExport}
                disabled={!framedImageUrl}
                className="btn btn-xs btn-primary"
                title="Export framed image"
              >
                <ArrowDownTrayIcon className="w-3 h-3" />
              </button>
              <button
                onClick={() => handleFrameSelect(selectedFrame)}
                className="btn btn-xs btn-ghost"
                title="Remove frame"
              >
                <XMarkIcon className="w-3 h-3" />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Frame Selection Grid */}
      <div>
        <div className="text-sm font-medium text-base-content/80 mb-3">
          Choose a Frame
        </div>
        <div className="grid grid-cols-2 gap-2">
          {allFrames.map((frame) => (
            <button
              key={frame.id}
              onClick={() => handleFrameSelect(frame)}
              className={`p-3 rounded-lg border text-left transition-all hover:shadow-sm ${
                selectedFrameId === frame.id
                  ? "border-primary bg-primary/10"
                  : "border-base-content/20 bg-base-100 hover:border-primary/50"
              }`}
            >
              <div className="flex items-center space-x-2">
                <PhotoIcon className="w-4 h-4 text-primary flex-shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="font-medium text-sm truncate">
                    {frame.name}
                  </div>
                  <div className="text-xs text-base-content/60 truncate">
                    {frame.tags.slice(0, 2).join(", ")}
                  </div>
                </div>
              </div>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
};
