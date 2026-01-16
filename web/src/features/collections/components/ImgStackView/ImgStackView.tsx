import { GalleryVerticalEnd } from "lucide-react";

interface ImgStackViewProps {
  coverImages?: string[];
}

function ImgStackView({ coverImages }: ImgStackViewProps) {
  const hasCover = coverImages && coverImages.length > 0;

  return (
    <div className="group relative inline-block size-50">
      {/* Back card */}
      <div className="absolute inset-0 translate-x-2 translate-y-2 -rotate-2 rounded-2xl border border-base-300/60 bg-gradient-to-br from-base-200 to-base-100 shadow-lg transition-transform duration-300 ease-out group-hover:translate-x-2.5 group-hover:translate-y-2.5" />

      {/* Middle card */}
      <div className="absolute inset-0 translate-x-1 translate-y-1 rotate-1 rounded-2xl border border-base-300/70 bg-gradient-to-br from-primary/10 via-base-100 to-secondary/10 shadow-xl transition-transform duration-300 ease-out group-hover:translate-x-1.5 group-hover:translate-y-1.5" />

      {/* Top card */}
      <div className="relative flex size-full items-center justify-center overflow-hidden rounded-2xl border border-base-300 bg-gradient-to-br from-base-100 via-base-200/60 to-base-100 shadow-2xl">
        {/* Soft gradient glow accents */}
        <div className="pointer-events-none absolute -left-8 -top-8 h-24 w-24 rounded-full bg-gradient-to-br from-primary/30 to-transparent blur-2xl opacity-70" />
        <div className="pointer-events-none absolute -right-8 -bottom-10 h-28 w-28 rounded-full bg-gradient-to-tr from-secondary/30 to-transparent blur-2xl opacity-70" />

        {hasCover ? (
          <img
            src={coverImages[0]}
            alt="Album cover"
            className="size-full object-cover"
            loading="lazy"
          />
        ) : (
          <GalleryVerticalEnd className="size-10 text-base-content/80" />
        )}

        {/* Subtle floating animation on hover */}
        <div className="pointer-events-none absolute inset-0 transition-transform duration-300 ease-out group-hover:-translate-y-0.5" />
      </div>
    </div>
  );
}

export default ImgStackView;
