import { GalleryVerticalEnd } from "lucide-react";
import { useState } from "react";

interface ImgStackViewProps {
  coverImages?: string[];
  albumName?: string;
  isSelected?: boolean;
}

function ImgStackView({ coverImages, albumName, isSelected = false }: ImgStackViewProps) {
  const hasCover = coverImages && coverImages.length > 0;
  const [imageError, setImageError] = useState(false);

  // Selection styles
  const selectionOpacity = isSelected ? "opacity-100" : "opacity-70";

  return (
    <div className={`group relative inline-block size-50 transition-all duration-300 ${isSelected ? 'scale-95' : ''}`}>
      {/* Back card */}
      <div className={`absolute inset-0 translate-x-2 translate-y-2 -rotate-2 rounded-2xl border border-base-300/60 bg-gradient-to-br from-base-200 to-base-100 shadow-lg transition-all duration-300 ease-out 
        ${isSelected ? 'translate-x-3 translate-y-3 -rotate-3 bg-primary/20 border-primary/30' : 'group-hover:translate-x-2.5 group-hover:translate-y-2.5'}
      `} />

      {/* Middle card */}
      <div className={`absolute inset-0 translate-x-1 translate-y-1 rotate-1 rounded-2xl border border-base-300/70 bg-gradient-to-br from-primary/10 via-base-100 to-secondary/10 shadow-xl transition-all duration-300 ease-out
        ${isSelected ? 'translate-x-1.5 translate-y-1.5 rotate-2 bg-primary/10 border-primary/40' : 'group-hover:translate-x-1.5 group-hover:translate-y-1.5'}
      `} />

      {/* Top card */}
      <div className={`relative flex size-full items-center justify-center overflow-hidden rounded-2xl border shadow-2xl transition-all duration-300
        ${isSelected 
          ? 'border-primary ring-4 ring-primary ring-inset bg-primary/5' 
          : 'border-base-300 bg-gradient-to-br from-base-100 via-base-200/60 to-base-100'}
      `}>
        {/* Soft gradient glow accents */}
        <div className={`pointer-events-none absolute -left-8 -top-8 h-24 w-24 rounded-full bg-gradient-to-br from-primary/30 to-transparent blur-2xl transition-opacity duration-300 ${selectionOpacity}`} />
        <div className={`pointer-events-none absolute -right-8 -bottom-10 h-28 w-28 rounded-full bg-gradient-to-tr from-secondary/30 to-transparent blur-2xl transition-opacity duration-300 ${selectionOpacity}`} />

        {hasCover && !imageError ? (
          <img
            src={coverImages[0]}
            alt={albumName ? `Cover image for ${albumName} album` : "Album cover image"}
            className={`size-full object-cover transition-all duration-300 ${isSelected ? 'brightness-90 scale-105' : ''}`}
            loading="lazy"
            onError={() => setImageError(true)}
          />
        ) : (
          <GalleryVerticalEnd className={`size-10 transition-colors duration-300 ${isSelected ? 'text-primary' : 'text-base-content/80'}`} />
        )}

        {/* Selection Overlay Tint */}
        {isSelected && (
          <div className="absolute inset-0 bg-primary/10 pointer-events-none animate-in fade-in duration-300" />
        )}

        {/* Subtle floating animation on hover */}
        <div className="pointer-events-none absolute inset-0 transition-transform duration-300 ease-out group-hover:-translate-y-0.5" />
      </div>
    </div>
  );
}

export default ImgStackView;
