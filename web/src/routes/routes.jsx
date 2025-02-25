import Home from "@/pages/Home"
import Photos from "@/pages/Photos"

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
    }
]