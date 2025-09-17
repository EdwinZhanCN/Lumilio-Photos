import Home from "@/features/home/routes/Home";
import Assets from "@/features/assets/routes/Assets";
import { Studio } from "@/features/studio/routes/Studio";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { Lumen } from "@/features/lumen/routes/Lumen";
import { LumenWikiExample } from "@/features/lumen/components/LumenWiki/LumenWikiExample";
import Settings from "@/features/settings/routes/Settings";
import Monitor from "@/features/monitor/routes/Monitor";
import UploadAssets from "@/features/upload/routes/UploadAssets";
import { Portfolio } from "@/features/portfolio";
import Collections from "@/features/collections/routes/Collections";
import Updates from "@/features/updates/routes/Updates";
import Search from "@/features/search/routes/Search";

export const routes = [
  {
    path: "/",
    element: <Home />,
  },
  {
    path: "/updates",
    element: <Updates />,
  },
  {
    path: "/settings",
    element: <Settings />,
  },
  {
    path: "/collections",
    element: <Collections />,
  },
  {
    path: "/assets/photos",
    element: <Assets />,
  },
  {
    path: "/assets/",
    element: <Assets />,
  },
  {
    path: "/assets/photos/:assetId",
    element: <Assets />,
  },
  {
    path: "/assets/videos",
    element: <Assets />,
  },
  {
    path: "/assets/videos/:assetId",
    element: <Assets />,
  },
  {
    path: "/assets/audios",
    element: <Assets />,
  },
  {
    path: "/assets/audios/:assetId",
    element: <Assets />,
  },
  {
    path: "/search/",
    element: <Search />,
  },
  {
    path: "/upload-photos",
    element: <UploadAssets />,
  },
  {
    path: "/studio",
    element: (
      <WorkerProvider preload={["exif", "border"]}>
        <Studio />
      </WorkerProvider>
    ),
  },
  {
    path: "/server-monitor",
    element: <Monitor />,
  },
  {
    path: "/lumen",
    element: <Lumen />,
  },
  {
    path: "/portfolio",
    element: <Portfolio />,
  },
  {
    path: "/test-lumen",
    element: (
      <WorkerProvider preload={["llm"]}>
        <LumenWikiExample />
      </WorkerProvider>
    ),
  },
];
