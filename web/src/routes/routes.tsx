import Home from "@/features/home/routes/Home";
import Assets from "@/features/assets/routes/Assets";
import { Studio } from "@/features/studio/routes/Studio";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import Settings from "@/features/settings/routes/Settings";
import Monitor from "@/features/monitor/routes/Monitor";
import UploadAssets from "@/features/upload/routes/UploadAssets";
import { Portfolio } from "@/features/portfolio";
import Collections from "@/features/collections/routes/Collections";
import AlbumDetails from "@/features/collections/routes/AlbumDetails";
import Updates from "@/features/updates/routes/Updates";
import LumilioChatPage from "@/features/lumilio/routes/LumilioChat";
import {AssetsProvider} from "@/features/assets";
import LoginPage from "@/features/auth/routes/LoginPage.tsx";
import RegisterPage from "@/features/auth/routes/RegisterPage.tsx";

export const routes = [
  {
    path: "/",
    element: <Home />,
  },
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/register",
    element: <RegisterPage />,
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
    path: "/collections/:albumId",
    element: <AlbumDetails />,
  },
  {
    path: "/collections/:albumId/:assetId",
    element: <AlbumDetails />,
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
    path: "/lumilio",
    element: (
        <AssetsProvider>
            <LumilioChatPage />
        </AssetsProvider>
    ),
  },
  {
    path: "/portfolio",
    element: <Portfolio />,
  },
];
