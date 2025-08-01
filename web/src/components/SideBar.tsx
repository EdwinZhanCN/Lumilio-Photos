import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  AdjustmentsHorizontalIcon,
  ArrowUpTrayIcon,
  ServerStackIcon,
  HomeIcon,
  InformationCircleIcon,
  PhotoIcon,
  PaintBrushIcon,
  ArchiveBoxIcon,
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
          <Link to="/" className={location.pathname === "/" ? "active" : ""}>
            <HomeIcon className="size-5" />
            Home
            <span className="badge badge-sm">{messageCount}</span>
          </Link>
        </li>
        <li>
          <Link
            to="/assets/"
            className={location.pathname.startsWith("/assets") ? "active" : ""}
          >
            <PhotoIcon className="size-5" />
            Assets
          </Link>
        </li>
        <li>
          <Link
            to="/collections"
            className={location.pathname === "/favorites" ? "active" : ""}
          >
            <ArchiveBoxIcon className="size-5" />
            Collections
          </Link>
        </li>
        <li>
          <Link to={"/studio"}>
            <PaintBrushIcon className="size-5" />
            Studio
          </Link>
        </li>
        <li>
          <Link to="/upload-photos">
            <ArrowUpTrayIcon className="size-5" />
            Upload
          </Link>
        </li>
        <li>
          <Link to="/settings">
            <AdjustmentsHorizontalIcon className="size-5" />
            Settings
          </Link>
        </li>
        <li>
          <Link
            to="/updates"
            className={location.pathname === "/updates" ? "active" : ""}
          >
            <InformationCircleIcon className="size-5" />
            Updates
            {isUpdate && (
              <span className="badge badge-sm badge-warning">NEW</span>
            )}
          </Link>
        </li>
        <li>
          <Link to="/server-monitor">
            {isOnline ? (
              <div className="flex items-center justify-center gap-2 text-success">
                <ServerStackIcon className="size-5" />
                Online
                <div className="inline-grid *:[grid-area:1/1]">
                  <div className="status status-success animate-ping"></div>
                  <div className="status status-success"></div>
                </div>
              </div>
            ) : (
              <div className="flex items-center justify-center gap-2 text-error">
                <ServerStackIcon className="size-5" />
                Offline
                <div className="inline-grid *:[grid-area:1/1]">
                  <div className="status status-error animate-ping"></div>
                  <div className="status status-error"></div>
                </div>
              </div>
            )}
          </Link>
        </li>
      </ul>
    </div>
  );
}

export default SideBar;
