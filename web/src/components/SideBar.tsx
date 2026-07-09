import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  Activity,
  Folders,
  Home,
  Image,
  LibraryBig,
  Moon,
  Paintbrush,
  SlidersHorizontal,
  Sun,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useGlobal } from "@/contexts/GlobalContext";
import { useAuth } from "@/features/auth";
import { useResolvedThemeMode, useThemePreference } from "@/lib/theme";
import UserAvatar from "@/components/UserAvatar";

/** Unchecks the shell drawer checkbox so navigating on mobile auto-closes it. */
function closeMobileDrawer() {
  const toggle = document.getElementById("app-drawer") as HTMLInputElement | null;
  if (toggle) toggle.checked = false;
}

function SideBar() {
  const { online: isOnline } = useGlobal();
  const { user, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const { t } = useI18n();
  const [theme, setTheme] = useThemePreference();
  const resolvedThemeMode = useResolvedThemeMode();
  const isDarkMode = resolvedThemeMode === "dark";
  const isFollowingSystem = theme.followSystem;
  const displayName = user?.display_name || user?.username || "User";

  return (
    <div className="flex h-full w-full min-w-0 select-none flex-col">
      <div className="shrink-0 p-2 pb-1">
        <Link
          to="/"
          onClick={closeMobileDrawer}
          className="btn btn-ghost h-auto min-h-0 w-full justify-start gap-2 px-3 py-2"
        >
          <img
            src="/logo.png"
            className="size-6 shrink-0 bg-contain object-contain"
            alt={t("app.name") + " Logo"}
          />
          <span className="truncate text-base font-semibold">{t("app.name")}</span>
        </Link>
      </div>

      <ul className="menu min-h-0 w-full flex-1 gap-1 overflow-y-auto rounded-none p-2 pt-1">
        <li>
          <Link
            to="/"
            onClick={closeMobileDrawer}
            className={location.pathname === "/" ? "active" : ""}
          >
            <Home className="size-5" />
            {t("sidebar.home")}
          </Link>
        </li>
        <li>
          <Link
            to="/assets/"
            onClick={closeMobileDrawer}
            className={location.pathname.startsWith("/assets") ? "active" : ""}
          >
            <Image className="size-5" />
            {t("sidebar.assets")}
          </Link>
        </li>
        <li>
          <Link
            to="/collections"
            onClick={closeMobileDrawer}
            className={location.pathname.startsWith("/collections") ? "active" : ""}
          >
            <LibraryBig className="size-5" />
            {t("sidebar.collections")}
          </Link>
        </li>
        <li>
          <Link
            to="/studio"
            onClick={closeMobileDrawer}
            className={location.pathname.startsWith("/studio") ? "active" : ""}
          >
            <Paintbrush className="size-5" />
            {t("sidebar.studio")}
          </Link>
        </li>
        <li>
          <Link
            to="/manage"
            onClick={closeMobileDrawer}
            className={location.pathname.startsWith("/manage") ? "active" : ""}
          >
            <Folders className="size-5" />
            {t("sidebar.manage")}
          </Link>
        </li>
        <li>
          <Link
            to="/settings"
            onClick={closeMobileDrawer}
            className={location.pathname.startsWith("/settings") ? "active" : ""}
          >
            <SlidersHorizontal className="size-5" />
            {t("sidebar.settings")}
          </Link>
        </li>
        {user?.role === "admin" && (
          <li>
            <Link
              to="/server-monitor"
              onClick={closeMobileDrawer}
              className={location.pathname.startsWith("/server-monitor") ? "active" : ""}
              title={
                isOnline
                  ? t("sidebar.status.online")
                  : t("sidebar.status.offline")
              }
            >
              <Activity className={`size-5 ${isOnline ? "text-success" : "text-error"}`} />
              {t("sidebar.status.label", { defaultValue: "Status" })}
              <span className="ml-auto inline-grid *:[grid-area:1/1]">
                <span
                  className={`status ${isOnline ? "status-success animate-ping" : "status-error animate-ping"}`}
                />
                <span className={`status ${isOnline ? "status-success" : "status-error"}`} />
              </span>
            </Link>
          </li>
        )}
      </ul>

      {user && (
        <div className="shrink-0 border-t border-base-300/60 p-2">
          <div className="dropdown dropdown-top dropdown-end w-full">
            <button
              type="button"
              tabIndex={0}
              className="btn btn-ghost h-auto min-h-0 w-full justify-start gap-2 px-3 py-2"
            >
              <UserAvatar
                assetId={user.avatar_asset_id}
                name={displayName}
                size="size-8"
                textSize="text-sm"
              />
              <div className="min-w-0 flex-1 text-left">
                <div className="truncate text-sm font-semibold leading-tight">{displayName}</div>
                <div className="truncate text-xs leading-tight opacity-60">
                  {(user.role ?? "user").toUpperCase()}
                </div>
              </div>
            </button>
            <ul
              tabIndex={0}
              className="menu dropdown-content z-20 mb-2 w-56 rounded-2xl border border-base-300 bg-base-100 p-2 shadow-xl"
            >
              <li className="menu-title px-3 py-2">
                <span className="font-semibold text-base-content">{displayName}</span>
                {user.username && (
                  <span className="text-xs text-base-content/70">@{user.username}</span>
                )}
              </li>
              <li>
                <Link to="/settings?tab=account" onClick={closeMobileDrawer}>
                  {t("settings.account.title", { defaultValue: "Account" })}
                </Link>
              </li>
              {user.role === "admin" && (
                <li>
                  <Link to="/settings?tab=users" onClick={closeMobileDrawer}>
                    {t("settings.users.title", { defaultValue: "Users" })}
                  </Link>
                </li>
              )}
              <li>
                <Link to="/settings?tab=appearance" onClick={closeMobileDrawer}>
                  {t("settings.appearance", { defaultValue: "Appearance" })}
                </Link>
              </li>
              <li>
                <button
                  type="button"
                  disabled={isFollowingSystem}
                  title={
                    isFollowingSystem
                      ? t("settings.appearanceSettings.themes.followSystem.navbarHint")
                      : undefined
                  }
                  className={isFollowingSystem ? "opacity-60" : undefined}
                  onClick={() => {
                    if (isFollowingSystem) return;
                    setTheme({
                      ...theme,
                      mode: isDarkMode ? "light" : "dark",
                    });
                  }}
                >
                  {isDarkMode ? <Sun className="size-4" /> : <Moon className="size-4" />}
                  {isDarkMode
                    ? t("sidebar.theme.useLight", { defaultValue: "Use light mode" })
                    : t("sidebar.theme.useDark", { defaultValue: "Use dark mode" })}
                </button>
              </li>
              <li>
                <button
                  type="button"
                  onClick={() => {
                    closeMobileDrawer();
                    void logout();
                    void navigate("/login", { replace: true });
                  }}
                >
                  {t("auth.logout", { defaultValue: "Logout" })}
                </button>
              </li>
            </ul>
          </div>
        </div>
      )}
    </div>
  );
}

export default SideBar;
