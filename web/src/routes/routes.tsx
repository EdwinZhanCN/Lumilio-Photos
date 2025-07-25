import Home from "@/pages/Home";
import Assets from "@/pages/Assets";
import UploadAssets from "@/pages/UploadAssets.tsx";
import { Studio } from "@/pages/Studio.tsx";
import { WorkerProvider } from "@/contexts/WorkerProvider";

export const routes = [
  {
    path: "/",
    element: <Home />,
  },
  {
    path: "/updates",
    element: <div>Updates Page</div>,
  },
  {
    path: "/settings",
    element: <div>Settings Page</div>,
  },
  {
    path: "/collections",
    element: <div>Collections Page</div>,
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
    path: "/upload-photos",
    element: <UploadAssets />,
  },
  {
    path: "/studio",
    element: (
      <WorkerProvider>
        <Studio />
      </WorkerProvider>
    ),
  },
  {
    path: "/server-monitor",
    element: <div>Server Monitor</div>,
  },
];
