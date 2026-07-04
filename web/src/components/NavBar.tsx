import { Link, useNavigate } from "react-router-dom";
import { Menu, Moon, Sun } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "@/features/auth";
import { useResolvedThemeMode, useThemePreference } from "@/lib/theme";
import UserAvatar from "@/components/UserAvatar";
import MessageCenter from "@/components/MessageCenter";
import NavbarUploadQueue from "@/features/upload/components/NavbarUploadQueue";

function NavBar() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  const [theme, setTheme] = useThemePreference();
  const resolvedThemeMode = useResolvedThemeMode();
  const isDarkMode = resolvedThemeMode === "dark";
  const isFollowingSystem = theme.followSystem;
  const displayName = user?.display_name || user?.username || "User";

  return (
    <div className="navbar bg-base-100 px-2 sm:px-4 py-2 gap-2 sm:gap-3 z-49">
      <div className="flex flex-1 items-center gap-1 sm:gap-3 min-w-0">
        <label
          htmlFor="app-drawer"
          aria-label={t("sidebar.openMenu", { defaultValue: "Open menu" })}
          className="btn btn-square btn-ghost lg:hidden"
        >
          <Menu className="size-5" />
        </label>
        <Link className="btn btn-ghost text-xl shrink-0 px-2 sm:px-4" to="/">
          <img
            src={"/logo.png"}
            className="size-6 bg-contain object-contain"
            alt={t("app.name") + " Logo"}
          />
          <span className="hidden sm:inline">{t("app.name")}</span>
        </Link>
      </div>

      <div className="flex flex-1 justify-end">
        <div className="flex items-center gap-1 sm:gap-3">
          <MessageCenter />
          <NavbarUploadQueue />
          <label
            className={`swap swap-rotate shrink-0 ${isFollowingSystem ? "cursor-not-allowed opacity-60" : ""}`}
            title={
              isFollowingSystem
                ? t("settings.appearanceSettings.themes.followSystem.navbarHint")
                : undefined
            }
          >
            <input
              type="checkbox"
              checked={isDarkMode}
              disabled={isFollowingSystem}
              onChange={(e) => {
                setTheme({
                  ...theme,
                  mode: e.target.checked ? "dark" : "light",
                });
              }}
            />

            <Sun className="swap-off h-5 w-5 sm:h-6 sm:w-6" />
            <Moon className="swap-on h-5 w-5 sm:h-6 sm:w-6" />
          </label>

          {user && (
            <div className="dropdown dropdown-end">
              <button
                type="button"
                tabIndex={0}
                className="btn btn-ghost h-auto min-h-0 rounded-full px-1 py-1 sm:px-2"
              >
                <UserAvatar
                  assetId={user.avatar_asset_id}
                  name={displayName}
                  size="size-8 sm:size-10"
                  textSize="text-sm"
                />
                <div className="hidden text-left sm:block">
                  <div className="text-sm font-semibold leading-tight">{displayName}</div>
                  <div className="text-xs opacity-60 leading-tight">
                    {(user.role ?? "user").toUpperCase()}
                  </div>
                </div>
              </button>
              <ul
                tabIndex={0}
                className="menu dropdown-content z-20 mt-2 w-64 rounded-2xl border border-base-300 bg-base-100 p-2 shadow-xl"
              >
                <li className="menu-title px-3 py-2">
                  <span className="font-semibold text-base-content">{displayName}</span>
                  {user.username && (
                    <span className="text-xs text-base-content/70">@{user.username}</span>
                  )}
                </li>
                <li>
                  <Link to="/settings?tab=account">
                    {t("settings.account.title", { defaultValue: "Account" })}
                  </Link>
                </li>
                {user.role === "admin" && (
                  <li>
                    <Link to="/settings?tab=users">
                      {t("settings.users.title", {
                        defaultValue: "Users",
                      })}
                    </Link>
                  </li>
                )}
                <li>
                  <button
                    type="button"
                    onClick={() => {
                      void logout();
                      void navigate("/login", { replace: true });
                    }}
                  >
                    {t("auth.logout", { defaultValue: "Logout" })}
                  </button>
                </li>
              </ul>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default NavBar;
