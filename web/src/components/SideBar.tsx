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
  BookOpenIcon,
} from "@heroicons/react/24/outline/index.js";

import { Album } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useGlobal } from "@/contexts/GlobalContext";

function SideBar() {
  const [messageCount] = useState<number>(0);
  const [isUpdate] = useState<boolean>(false);
  const { online: isOnline } = useGlobal();
  const location = useLocation();
  const { t } = useI18n();

  return (
    <div className="select-none">
      <ul className="menu rounded-box mx-2 my-2 gap-2">
        <li>
          <Link to="/" className={location.pathname === "/" ? "active" : ""}>
            <HomeIcon className="size-5" />
            {t("sidebar.home")}
            <span className="badge badge-sm">{messageCount}</span>
          </Link>
        </li>
        <li>
          <Link
            to="/assets/"
            className={location.pathname.startsWith("/assets") ? "active" : ""}
          >
            <PhotoIcon className="size-5" />
            {t("sidebar.assets")}
          </Link>
        </li>
        <li>
          <Link
            to="/collections"
            className={location.pathname === "/favorites" ? "active" : ""}
          >
            <Album size={20} strokeWidth={1.5} />
            {t("sidebar.collections")}
          </Link>
        </li>
        <li>
          <Link to={"/studio"}>
            <PaintBrushIcon className="size-5" />
            {t("sidebar.studio")}
          </Link>
        </li>
        <li>
          <Link to={"/portfolio"}>
            <BookOpenIcon className="size-5" />
            {t("sidebar.portfolio")}
          </Link>
        </li>
        <li>
          <div className="bg-linear-50 from-ctp-maroon-300 text-ctp-base">
            <ArrowUpTrayIcon className="size-5" />
            <Link to="/upload-photos">{t("sidebar.upload")}</Link>
          </div>
        </li>
        <li>
          <Link to="/settings">
            <AdjustmentsHorizontalIcon className="size-5" />
            {t("sidebar.settings")}
          </Link>
        </li>
        <li>
          <Link
            to="/updates"
            className={location.pathname === "/updates" ? "active" : ""}
          >
            <InformationCircleIcon className="size-5" />
            {t("sidebar.updates")}
            {isUpdate && (
              <span className="badge badge-sm badge-warning">
                {t("sidebar.badges.new")}
              </span>
            )}
          </Link>
        </li>
        <li>
          <Link to="/server-monitor">
            {isOnline ? (
              <div className="flex items-center justify-center gap-2 text-success">
                <ServerStackIcon className="size-5" />
                {t("sidebar.status.online")}
                <div className="inline-grid *:[grid-area:1/1]">
                  <div className="status status-success animate-ping"></div>
                  <div className="status status-success"></div>
                </div>
              </div>
            ) : (
              <div className="flex items-center justify-center gap-2 text-error">
                <ServerStackIcon className="size-5" />
                {t("sidebar.status.offline")}
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
