import Home from "@/features/home/routes/Home";
import Assets from "@/features/assets/routes/Assets";
import { Studio } from "@/features/studio/routes/Studio";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import Settings from "@/features/settings/routes/Settings";
import Monitor from "@/features/monitor/routes/Monitor";
import Manage from "@/features/manage/routes/Manage";
import Collections from "@/features/collections/routes/Collections";
import Albums from "@/features/collections/routes/Albums";
import AlbumDetails from "@/features/collections/routes/AlbumDetails";
import MapView from "@/features/collections/routes/MapView";
import TripDetails from "@/features/collections/routes/TripDetails";
import People from "@/features/collections/routes/People";
import Duplicates from "@/features/collections/routes/Duplicates";
import PersonDetails from "@/features/people/routes/PersonDetails";
import LumilioChatPage from "@/features/lumilio/routes/LumilioChat";
import { AssetsProvider } from "@/features/assets";
import LoginPage from "@/features/auth/routes/LoginPage.tsx";
import MFAPage from "@/features/auth/routes/MFAPage.tsx";
import ChangePasswordPage from "@/features/auth/routes/ChangePasswordPage.tsx";
import RegisterPage from "@/features/auth/routes/RegisterPage.tsx";
import { Navigate, useParams } from "react-router-dom";

const LegacyAssetDetailRedirect = () => {
  const { assetId } = useParams<{ assetId: string }>();
  return <Navigate to={assetId ? `/assets/${assetId}` : "/assets"} replace />;
};

export const publicRoutes = [
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/register",
    element: <RegisterPage />,
  },
];

export const protectedStandaloneRoutes = [
  {
    path: "/mfa",
    element: <MFAPage />,
  },
  {
    path: "/change-password",
    element: <ChangePasswordPage />,
  },
];

export const appRoutes = [
  {
    path: "/",
    element: <Home />,
  },
  // {
  //   path: "/updates",
  //   element: <Updates />,
  // },
  {
    path: "/settings",
    element: <Settings />,
  },
  {
    path: "/collections",
    element: <Collections />,
  },
  {
    path: "/collections/albums",
    element: <Albums />,
  },
  {
    path: "/collections/map",
    element: <MapView />,
  },
  {
    path: "/collections/places/:tripId",
    element: <TripDetails />,
  },
  {
    path: "/collections/places/:tripId/:assetId",
    element: <TripDetails />,
  },
  {
    path: "/collections/people",
    element: <People />,
  },
  {
    path: "/collections/utilities/duplicates",
    element: <Duplicates />,
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
    path: "/people/:personId",
    element: <PersonDetails />,
  },
  {
    path: "/people/:personId/:assetId",
    element: <PersonDetails />,
  },
  {
    path: "/assets",
    element: <Assets />,
  },
  {
    path: "/assets/:assetId",
    element: <Assets />,
  },
  {
    path: "/assets/photos",
    element: <Navigate to="/assets" replace />,
  },
  {
    path: "/assets/videos",
    element: <Navigate to="/assets" replace />,
  },
  {
    path: "/assets/audios",
    element: <Navigate to="/assets" replace />,
  },
  {
    path: "/assets/photos/:assetId",
    element: <LegacyAssetDetailRedirect />,
  },
  {
    path: "/assets/videos/:assetId",
    element: <LegacyAssetDetailRedirect />,
  },
  {
    path: "/assets/audios/:assetId",
    element: <LegacyAssetDetailRedirect />,
  },
  {
    path: "/manage",
    element: <Manage />,
  },
  {
    path: "/upload-photos",
    element: <Navigate to="/manage" replace />,
  },
  {
    path: "/studio",
    element: (
      <WorkerProvider preload={["exif", "plugin"]}>
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
      <AssetsProvider scopeId="lumilio">
        <LumilioChatPage />
      </AssetsProvider>
    ),
  },
  // {
  //   path: "/portfolio",
  //   element: <Portfolio />,
  // },
];
