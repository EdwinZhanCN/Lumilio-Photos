import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { FolderIcon } from "@heroicons/react/24/outline";
import { Moon, Sun } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "@/features/auth";
import { LumilioAvatar } from "@/features/lumilio/components/LumilioAvatar/LumilioAvatar";
import { useSettingsContext, useWorkingRepository } from "@/features/settings";

function getInitials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? "")
      .join("") || "U"
  );
}

function NavBar() {
  const [isLumilioHovered, setIsLumilioHovered] = useState(false);
  const { t } = useI18n();
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  const { state, dispatch, resolvedThemeMode } = useSettingsContext();
  const {
    repositories,
    repositoriesQuery,
    workingRepositoryId,
    selectedRepository,
    setWorkingRepositoryId,
    getRepositoryLabel,
  } = useWorkingRepository();
  const isDarkMode = resolvedThemeMode === "dark";
  const isFollowingSystem = state.ui.theme.followSystem;
  const displayName = user?.display_name || user?.username || "User";

  return (
    <div className="navbar bg-base-100 px-4 py-2 gap-3">
      <div className="flex flex-1 items-center gap-3 min-w-0">
        <Link className="btn btn-ghost text-xl flex-shrink-0" to="/">
          <img
            src={"/logo.png"}
            className="size-6 bg-contain object-contain"
            alt={t("app.name") + " Logo"}
          />
          {t("app.name")}
        </Link>

        <label className="form-control gap-1 min-w-0">
          <span className="sr-only">
            {t("navbar.repository.label", {
              defaultValue: "Working repository",
            })}
          </span>
          <div className="flex items-center gap-2 min-w-0">
            <select
              className="select select-bordered select-sm w-32 sm:w-52"
              value={workingRepositoryId}
              disabled={
                repositoriesQuery.isLoading || repositoriesQuery.isError
              }
              title={selectedRepository?.path}
              onChange={(event) =>
                setWorkingRepositoryId(event.target.value || null)
              }
            >
              <option value="">
                {t("navbar.repository.all", {
                  defaultValue: "All repositories",
                })}
              </option>
              {repositories.map((repository) => (
                <option key={repository.id} value={repository.id}>
                  {getRepositoryLabel(repository)}
                </option>
              ))}
            </select>
            <FolderIcon className="size-5 shrink-0 text-base-content/60" />
          </div>
          {repositoriesQuery.isError && (
            <span className="text-xs text-base-content/60">
              {t("navbar.repository.unavailable", {
                defaultValue: "Repository options unavailable",
              })}
            </span>
          )}
        </label>
      </div>

      <div className="flex flex-1 justify-center">
        <div
          className="tooltip tooltip-bottom"
          data-tip={t("navbar.agent.open")}
        >
          <Link
            to="/lumilio"
            className="inline-flex items-center justify-center rounded-full p-1"
            aria-label={t("navbar.agent.label")}
            onMouseEnter={() => setIsLumilioHovered(true)}
            onMouseLeave={() => setIsLumilioHovered(false)}
          >
            <LumilioAvatar
              className="mb-2"
              size={0.2}
              start={isLumilioHovered}
            />
          </Link>
        </div>
      </div>

      <div className="flex flex-1 justify-end">
        <div className="flex items-center gap-3">
          <label
            className={`swap swap-rotate ${isFollowingSystem ? "cursor-not-allowed opacity-60" : ""}`}
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
                dispatch({
                  type: "SET_THEME_MODE",
                  payload: e.target.checked ? "dark" : "light",
                });
              }}
            />

            <Sun className="swap-off h-6 w-6" />
            <Moon className="swap-on h-6 w-6" />
          </label>

          {user && (
            <div className="dropdown dropdown-end">
              <button
                type="button"
                tabIndex={0}
                className="btn btn-ghost h-auto min-h-0 rounded-full px-2 py-1"
              >
                <div className="avatar placeholder">
                  <div className="bg-primary text-primary-content size-10 rounded-full">
                    {user.avatar_url ? (
                      <img
                        src={user.avatar_url}
                        alt={displayName}
                        className="size-10 rounded-full object-cover"
                      />
                    ) : (
                      <span className="text-sm font-semibold">
                        {getInitials(displayName)}
                      </span>
                    )}
                  </div>
                </div>
                <div className="hidden text-left sm:block">
                  <div className="text-sm font-semibold leading-tight">
                    {displayName}
                  </div>
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
                  <span>{user.email}</span>
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
                      logout();
                      navigate("/login", { replace: true });
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
