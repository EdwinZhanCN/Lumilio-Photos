import { useMemo, useRef, useEffect, useState } from "react";
import { Swiper, SwiperSlide } from "swiper/react";
import { Virtual, Navigation, Pagination } from "swiper/modules";
import { XMarkIcon } from "@heroicons/react/24/outline";
import FullScreenToolbar from "../FullScreenToolbar/FullScreenToolbar";
import FullScreenInfo from "../FullScreenInfo/FullScreenInfo";
import "@/styles/custom-swiper.css";
import "swiper/css";
import "swiper/css/navigation";
import "swiper/css/pagination";
import { assetService } from "@/services/assetsService";
import FullScreenBasicInfo from "../FullScreenInfo/FullScreenBasicInfo";

interface FullScreenCarouselProps {
  photos: Asset[];
  initialSlide: number;
  onClose: () => void;
  onNavigate: (assetId: string) => void;
}

const FullScreenCarousel = ({
  photos,
  initialSlide,
  onClose,
  onNavigate,
}: FullScreenCarouselProps) => {
  const swiperRef = useRef<any>(null);
  const [showInfo, setShowInfo] = useState(false);
  const [currentAsset, setCurrentAsset] = useState(photos[initialSlide]);

  const slides = useMemo(() => {
    return photos.map((photo) => {
      const largeImageUrl = photo.asset_id
        ? assetService.getThumbnailUrl(photo.asset_id, "large")
        : undefined;
      return {
        src: largeImageUrl || "",
        alt: photo.original_filename || "Asset",
        assetId: photo.asset_id!,
      };
    });
  }, [photos]);

  useEffect(() => {
    if (swiperRef.current && swiperRef.current.swiper) {
      swiperRef.current.swiper.slideTo(initialSlide, 0);
    }
    setCurrentAsset(photos[initialSlide]);
  }, [initialSlide, photos]);

  const onSlideChange = (swiper: any) => {
    const assetId = slides[swiper.activeIndex]?.assetId;
    if (assetId) {
      onNavigate(assetId);
      setCurrentAsset(photos[swiper.activeIndex]);
    }
  };

  const toggleInfo = () => {
    setShowInfo(!showInfo);
  };

  return (
    <div className="fixed inset-0 bg-black/90 z-50 flex items-center justify-center animate-fade-in">
      <FullScreenToolbar
        onToggleInfo={toggleInfo}
        currentAsset={currentAsset}
      />
      <button
        onClick={onClose}
        className="btn btn-ghost btn-sm absolute top-2 left-4 text-white z-20"
      >
        <XMarkIcon className="w-6 h-6" />
      </button>
      <FullScreenBasicInfo />
      <Swiper
        ref={swiperRef}
        modules={[Virtual, Navigation, Pagination]}
        spaceBetween={50}
        slidesPerView={1}
        virtual
        navigation
        pagination={{ clickable: true }}
        onSlideChange={onSlideChange}
        initialSlide={initialSlide}
        className="fullscreen-swiper"
      >
        {slides.map((slide, index) => (
          <SwiperSlide key={slide.assetId} virtualIndex={index}>
            <div className="h-screen w-screen flex items-center justify-center p-4">
              <img
                src={slide.src}
                alt={slide.alt}
                className="max-h-full max-w-full object-contain select-none"
              />
            </div>
          </SwiperSlide>
        ))}
      </Swiper>
      {showInfo && currentAsset && <FullScreenInfo asset={currentAsset} />}
    </div>
  );
};

export default FullScreenCarousel;
