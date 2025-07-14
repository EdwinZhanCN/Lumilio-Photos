import { useState } from "react";
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
  const [activeTab, setActiveTab] = useState("photos");

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
            <li onClick={() => setActiveTab("photos")}>
              <a className={activeTab === "photos" ? "active" : ""}>
                <PhotoIcon className="h-5 w-5" />
                Photos
              </a>
            </li>
            <li onClick={() => setActiveTab("videos")}>
              <a className={activeTab === "videos" ? "active" : ""}>
                <VideoCameraIcon className="h-5 w-5" />
                Videos
              </a>
            </li>
            <li onClick={() => setActiveTab("audios")}>
              <a className={activeTab === "audios" ? "active" : ""}>
                <MusicalNoteIcon className="h-5 w-5" />
                Audios
              </a>
            </li>
          </ul>
        </div>
      </AssetsProvider>
    </ErrorBoundary>
  );
};

export default Assets;
