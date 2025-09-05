import { Link, useLocation } from "react-router-dom";
import {
  PhotoIcon,
  VideoCameraIcon,
  MusicalNoteIcon,
} from "@heroicons/react/24/solid";
import { useI18n } from "@/lib/i18n.tsx";

interface AssetTabsProps {
  isCarouselOpen: boolean;
}

const AssetTabs = ({ isCarouselOpen }: AssetTabsProps) => {
  const location = useLocation();
  const { t } = useI18n();

  // Determine active tab based on URL path
  const getActiveTab = () => {
    const path = location.pathname;
    if (path.includes("/videos")) {
      return "videos";
    } else if (path.includes("/audios")) {
      return "audios";
    } else {
      return "photos";
    }
  };

  const activeTab = getActiveTab();

  return (
    <div
      className={`fixed bottom-10 left-1/2 -translate-x-1/2 z-50 transition-all duration-300 ${
        isCarouselOpen
          ? "animate-fade-out-y opacity-0 pointer-events-none"
          : "animate-fade-in-y opacity-100 pointer-events-auto"
      }`}
    >
      <ul className="menu menu-horizontal glass rounded-box shadow-lg">
        <li>
          <Link
            to="/assets/photos"
            className={activeTab === "photos" ? "active" : ""}
          >
            <PhotoIcon className="h-5 w-5" />
            {t("assets.photos")}
          </Link>
        </li>
        <li>
          <Link
            to="/assets/videos"
            className={activeTab === "videos" ? "active" : ""}
          >
            <VideoCameraIcon className="h-5 w-5" />
            {t("assets.videos")}
          </Link>
        </li>
        <li>
          <Link
            to="/assets/audios"
            className={activeTab === "audios" ? "active" : ""}
          >
            <MusicalNoteIcon className="h-5 w-5" />
            {t("assets.audios")}
          </Link>
        </li>
      </ul>
    </div>
  );
};

export default AssetTabs;
