import { useState, useEffect } from "react";
import { useLocation, Link } from "react-router-dom";
import Photos from "./Photos";
import Videos from "./Videos";
import Audios from "./Audios";
import AssetsProvider from "@/contexts/FetchContext";
import ErrorBoundary from "@/ErrorBoundary";
import {
  PhotoIcon,
  VideoCameraIcon,
  MusicalNoteIcon,
} from "@heroicons/react/24/solid";

const Assets = () => {
  const location = useLocation();
  const [activeTab, setActiveTab] = useState("photos");

  // Determine active tab based on URL path
  useEffect(() => {
    const path = location.pathname;
    if (path.includes("/videos")) {
      setActiveTab("videos");
    } else if (path.includes("/audios")) {
      setActiveTab("audios");
    } else {
      // Default to photos for /assets/ or /assets/photos
      setActiveTab("photos");
    }
  }, [location.pathname]);

  return (
    <ErrorBoundary>
      <AssetsProvider>
        <div className="p-4 w-full max-w-screen-lg mx-auto mb-20">
          <div className="pt-4">
            {activeTab === "photos" && <Photos />}
            {activeTab === "videos" && <Videos />}
            {activeTab === "audios" && <Audios />}
          </div>
        </div>
        <div className="fixed bottom-10 left-1/2 -translate-x-1/2 z-50">
          <ul className="menu menu-horizontal bg-base-200 rounded-box shadow-lg">
            <li>
              <Link
                to="/assets/photos"
                className={activeTab === "photos" ? "active" : ""}
              >
                <PhotoIcon className="h-5 w-5" />
                Photos
              </Link>
            </li>
            <li>
              <Link
                to="/assets/videos"
                className={activeTab === "videos" ? "active" : ""}
              >
                <VideoCameraIcon className="h-5 w-5" />
                Videos
              </Link>
            </li>
            <li>
              <Link
                to="/assets/audios"
                className={activeTab === "audios" ? "active" : ""}
              >
                <MusicalNoteIcon className="h-5 w-5" />
                Audios
              </Link>
            </li>
          </ul>
        </div>
      </AssetsProvider>
    </ErrorBoundary>
  );
};

export default Assets;
