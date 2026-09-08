import { useAuth } from "@/features/auth";
import type { ReactNode } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { BreadcrumbProvider } from "@/components/breadcrumbs";
import { ChatDock } from "@/features/lumilio";
import {
  MusicNavigation,
  MusicPlayerDock,
  MusicPlayerProvider,
  useMusicPlayer,
} from "@/features/music";
import { useI18n } from "@/lib/i18n.tsx";
import NavBar from "@/app/shell/NavBar";
import SideBar from "@/app/shell/SideBar";

/** Shared navigation shell for authenticated application routes. */
export default function AppShellLayout(): ReactNode {
  const { user } = useAuth();
  return (
    <BreadcrumbProvider>
      <MusicPlayerProvider key={user?.user_id} ownerId={user?.user_id}>
        <AppShellContent />
      </MusicPlayerProvider>
    </BreadcrumbProvider>
  );
}

function AppShellContent(): ReactNode {
  const { t } = useI18n();
  const location = useLocation();
  const { current, isLoading, error } = useMusicPlayer();
  // The /lumilio board embeds its own dock; everywhere else gets the global
  // agent drawer (launched from the NavBar button, see AgentDockLauncher).
  const isMusic = location.pathname === "/music" || location.pathname.startsWith("/music/");
  const showAgentDock = location.pathname !== "/lumilio" && !isMusic;
  const hasPlayer = Boolean(current || isLoading || error);

  return (
    <div className={`${isMusic ? "" : "drawer lg:drawer-open"} h-screen`}>
      <input id="app-drawer" type="checkbox" className="drawer-toggle" />
      <div className="drawer-content flex h-screen min-h-0 flex-col overflow-hidden">
        {isMusic ? <MusicNavigation /> : <NavBar />}
        <div
          id="app-scroll-container"
          className={`flex min-h-0 flex-1 flex-col overflow-hidden ${hasPlayer ? "pb-16" : ""}`}
        >
          <div className="min-h-0 flex-1 overflow-hidden">
            <Outlet />
          </div>
        </div>
        <MusicPlayerDock />
        {showAgentDock && <ChatDock variant="fab" />}
      </div>
      {!isMusic && (
        <div className="drawer-side z-overlay lg:z-auto">
          <label
            htmlFor="app-drawer"
            aria-label={t("sidebar.closeMenu", { defaultValue: "Close menu" })}
            className="drawer-overlay"
          />
          <div className="flex h-full w-64 flex-col overflow-hidden bg-base-200 shadow-lg lg:w-56">
            <SideBar />
          </div>
        </div>
      )}
    </div>
  );
}
