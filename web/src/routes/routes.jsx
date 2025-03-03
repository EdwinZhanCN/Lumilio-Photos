import Home from "@/pages/Home"
import Photos from "@/pages/Photos"
import UploadPhotos from "@/pages/UploadPhotos.jsx";

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
        element: <UploadPhotos />,
    }
]