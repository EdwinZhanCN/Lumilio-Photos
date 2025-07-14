import Home from "@/pages/Home"
import Assets from "@/pages/Assets"
import UploadAssets from "@/pages/UploadAssets.tsx";
import {Studio} from "@/pages/Studio.tsx";

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
        path: "/photos",
        element: <Assets />,
    },
    {
        path: "/videos",
        element: <Assets />,
    },
    {
        path: "/audios",
        element: <Assets />,
    },
    {
        path: "/upload-photos",
        element: <UploadAssets />,
    },
    {
        path: "/studio",
        element: <Studio />,
    }
]