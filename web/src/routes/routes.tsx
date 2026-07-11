import { lazy } from "react";
import Home from "@/features/home/routes/Home";
import Assets from "@/features/assets/routes/Assets";
import AssetsTrash from "@/features/assets/routes/AssetsTrash";
import Manage from "@/features/manage/routes/Manage";
import Collections from "@/features/collections/routes/Collections";
import Albums from "@/features/collections/routes/Albums";
import AlbumDetails from "@/features/collections/routes/AlbumDetails";
import TripDetails from "@/features/collections/routes/TripDetails";
import People from "@/features/collections/routes/People";
import Utilities from "@/features/collections/routes/Utilities";
import Duplicates from "@/features/collections/routes/Duplicates";
import Liked from "@/features/collections/routes/Liked";
import UtilityClassifierAlbum from "@/features/collections/routes/UtilityClassifierAlbum";
import Folders from "@/features/collections/routes/Folders";
import FolderDetails from "@/features/collections/routes/FolderDetails";
import Tags from "@/features/collections/routes/Tags";
import TagDetails from "@/features/collections/routes/TagDetails";
import PersonDetails from "@/features/people/routes/PersonDetails";
import PublicShare from "@/features/share/routes/PublicShare.tsx";
import SharedLinks from "@/features/share/routes/SharedLinks.tsx";
import LoginPage from "@/features/auth/routes/LoginPage.tsx";
import MFAPage from "@/features/auth/routes/MFAPage.tsx";
import ChangePasswordPage from "@/features/auth/routes/ChangePasswordPage.tsx";
import RegisterPage from "@/features/auth/routes/RegisterPage.tsx";
import BootstrapWizard from "@/features/auth/routes/BootstrapWizard.tsx";
import { Navigate } from "react-router-dom";

const StudioEditMvp = lazy(() =>
  import("@/features/studio/routes/StudioEditMvp").then((module) => ({
    default: module.StudioEditMvp,
  })),
);
const Settings = lazy(() => import("@/features/settings/routes/Settings"));
const Monitor = lazy(() => import("@/features/monitor/routes/Monitor"));
const MapView = lazy(() => import("@/features/collections/routes/MapView"));
const LumilioChatPage = lazy(() => import("@/features/lumilio/routes/LumilioChat"));

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

// shareRoutes are rendered outside SetupGate/BootstrapGate (see App.tsx) so a
// recipient hitting a public share link is never redirected through
// first-run setup or forced to authenticate.
export const shareRoutes = [
  {
    path: "/s/:token",
    element: <PublicShare />,
  },
  {
    path: "/s/:token/:assetId",
    element: <PublicShare />,
  },
];

export const bootstrapRoutes = [
  {
    path: "/bootstrap",
    element: <BootstrapWizard />,
  },
  {
    path: "/bootstrap/register",
    element: <Navigate to="/bootstrap" replace />,
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
    path: "/collections/utilities",
    element: <Utilities />,
  },
  {
    path: "/collections/utilities/duplicates",
    element: <Duplicates />,
  },
  {
    path: "/collections/utilities/:classifierSlug",
    element: <UtilityClassifierAlbum />,
  },
  {
    path: "/collections/utilities/:classifierSlug/:assetId",
    element: <UtilityClassifierAlbum />,
  },
  {
    path: "/collections/folders",
    element: <Folders />,
  },
  {
    path: "/collections/folders/:folderKey",
    element: <FolderDetails />,
  },
  {
    path: "/collections/folders/:folderKey/:assetId",
    element: <FolderDetails />,
  },
  {
    path: "/collections/tags",
    element: <Tags />,
  },
  {
    path: "/collections/tags/:tagKey",
    element: <TagDetails />,
  },
  {
    path: "/collections/tags/:tagKey/:assetId",
    element: <TagDetails />,
  },
  {
    path: "/collections/trash",
    element: <AssetsTrash />,
  },
  {
    path: "/collections/trash/:assetId",
    element: <AssetsTrash />,
  },
  {
    path: "/collections/liked",
    element: <Liked />,
  },
  {
    path: "/collections/liked/:assetId",
    element: <Liked />,
  },
  {
    path: "/collections/shared-links",
    element: <SharedLinks />,
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
    path: "/manage",
    element: <Manage />,
  },
  {
    path: "/upload-photos",
    element: <Navigate to="/manage" replace />,
  },
  {
    path: "/studio",
    element: <StudioEditMvp />,
  },
  {
    path: "/server-monitor",
    element: <Monitor />,
  },
  {
    path: "/lumilio",
    element: <LumilioChatPage />,
  },
  // {
  //   path: "/portfolio",
  //   element: <Portfolio />,
  // },
];
