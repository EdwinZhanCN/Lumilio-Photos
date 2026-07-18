import type { ReactNode } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { BreadcrumbProvider } from "@/components/breadcrumbs";
import { ChatDock } from "@/features/lumilio";
import { useI18n } from "@/lib/i18n.tsx";
import NavBar from "@/app/shell/NavBar";
import SideBar from "@/app/shell/SideBar";

/** Shared navigation shell for authenticated application routes. */
export default function AppShellLayout(): ReactNode {
  const { t } = useI18n();
  const location = useLocation();
  // The /lumilio board embeds its own dock; everywhere else gets the global FAB.
  const showAgentDock = location.pathname !== "/lumilio";

  return (
    <BreadcrumbProvider>
      <div className="drawer lg:drawer-open h-screen">
        <input id="app-drawer" type="checkbox" className="drawer-toggle" />
        <div className="drawer-content flex h-screen min-h-0 flex-col overflow-hidden">
          <NavBar />
          <div id="app-scroll-container" className="flex min-h-0 flex-1 flex-col overflow-hidden">
            <div className="min-h-0 flex-1 overflow-hidden">
              <Outlet />
            </div>
          </div>
          {showAgentDock && <ChatDock variant="fab" />}
        </div>
        <div className="drawer-side z-50">
          <label
            htmlFor="app-drawer"
            aria-label={t("sidebar.closeMenu", { defaultValue: "Close menu" })}
            className="drawer-overlay"
          />
          <div className="flex h-full w-64 flex-col overflow-hidden bg-base-200 shadow-lg lg:w-56">
            <SideBar />
          </div>
        </div>
      </div>
    </BreadcrumbProvider>
  );
}
