import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import {
    AdjustmentsHorizontalIcon, ArrowUpTrayIcon,
    HeartIcon,
    HomeIcon,
    InformationCircleIcon,
    PhotoIcon,
    PaintBrushIcon
} from "@heroicons/react/24/outline/index.js";

function SideBar() {
    const [messageCount] = useState<number>(0);
    const [isUpdate] = useState<boolean>(false);
    const [isOnline] = useState<boolean>(false);
    const location = useLocation();

    return (
        <div className="select-none">
            <ul className="menu bg-base-200 rounded-box mx-2 my-2 gap-2">
                <li>
                    <Link
                        to="/"
                        className={location.pathname === "/" ? "active" : ""}>
                        <HomeIcon className="size-5"/>
                        Home
                        <span className="badge badge-sm">{messageCount}</span>
                    </Link>
                </li>
                <li>
                <Link
                    to="/photos"
                    className={location.pathname === "/photos" ? "active" : ""}
                >
                    <PhotoIcon className="size-5"/>
                    Photos
                </Link>
                </li>
                <li>
                    <Link
                        to="/favorites"
                        className={location.pathname === "/favorites" ? "active" : ""}>
                        <HeartIcon className="size-5"/>
                        Favorites
                    </Link>
                </li>
                <li>
                    <Link
                        to="/updates"
                        className={location.pathname === "/updates" ? "active" : ""} >
                        <InformationCircleIcon className="size-5"/>
                        Updates
                        {isUpdate && (
                            <span className="badge badge-sm badge-warning">NEW</span>
                        )}
                    </Link>
                </li>
                <li>
                    <Link
                        to="/settings"
                    >
                        <AdjustmentsHorizontalIcon className="size-5"/>
                        Settings
                    </Link>

                </li>
                <li>
                    <Link
                        to="/upload-photos"
                    >
                        <ArrowUpTrayIcon className="size-5"/>
                        Upload
                    </Link>
                </li>
                <li>
                    <Link to={"/studio"}>
                        <PaintBrushIcon className="size-5"/>
                        Studio
                    </Link>
                </li>
                <li>
                    <a>
                        {isOnline ? (
                            <div className="flex items-center gap-1">
                                <div className="inline-grid *:[grid-area:1/1]">
                                    <div className="status status-success animate-ping"></div>
                                    <div className="status status-success"></div>
                                </div> Server Online
                            </div>
                        ) : (
                            <div className="flex items-center gap-1">
                                <div className="inline-grid *:[grid-area:1/1]">
                                    <div className="status status-error animate-ping"></div>
                                    <div className="status status-error"></div>
                                </div> Server Offline
                            </div>
                        )}
                    </a>
                </li>

            </ul>
        </div>
);
}

export default SideBar;