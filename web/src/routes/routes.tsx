import Home from "@/pages/Home"
import Photos from "@/pages/Photos"
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
        element: <Photos />,
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