import React, { useEffect } from "react";
import { BrowserRouter, Outlet, Route, Routes, useLocation } from "react-router-dom";
import SideBar from "@/components/SideBar";
import NavBar from "@/components/NavBar";
import { ChatDock } from "@/features/lumilio/components/Chat/ChatDock";
import {
  appRoutes,
  bootstrapRoutes,
  protectedStandaloneRoutes,
  publicRoutes,
  shareRoutes,
} from "@/routes/routes";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import GlobalProvider, { useGlobal } from "@/contexts/GlobalContext";
import "@/styles/App.css";
import "katex/dist/katex.min.css";
import "streamdown/styles.css";
import Notifications from "@/components/Notifications";
import { PreferencesEffects, usePreference } from "./features/settings";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import {
  AuthProvider,
  BootstrapGate,
  PrimaryRepositoryGate,
  ProtectedRoute,
  SetupGate,
} from "./features/auth";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { UploadProvider } from "@/features/upload";
import { BreadcrumbProvider } from "@/components/breadcrumbs";

const queryClient = new QueryClient();

function AppShellLayout(): React.ReactNode {
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

function HealthPoller(): React.ReactNode {
  const [healthCheckIntervalMs] = usePreference("healthCheckIntervalMs");
  const { setOnline } = useGlobal();

  const intervalMs = Math.max(1000, Math.min(50_000, Math.max(1000, healthCheckIntervalMs)));

  const healthQuery = $api.useQuery(
    "get",
    "/api/v1/health",
    {},
    {
      refetchInterval: intervalMs,
      refetchIntervalInBackground: true,
      retry: false,
    },
  );

  useEffect(() => {
    if (healthQuery.isSuccess) {
      setOnline(true);
      return;
    }
    if (healthQuery.isError) {
      setOnline(false);
    }
  }, [healthQuery.isSuccess, healthQuery.isError, setOnline]);

  return null;
}

function App(): React.ReactNode {
  return (
    <PreferencesEffects>
      <GlobalProvider>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <BrowserRouter>
              <Routes>
                {/* Public share routes render outside SetupGate/BootstrapGate:
                    a recipient with a valid token must never be redirected
                    through first-run setup or forced to authenticate. */}
                {shareRoutes.map((route) => (
                  <Route key={route.path} path={route.path} element={route.element} />
                ))}
                <Route
                  element={
                    <SetupGate>
                      <BootstrapGate>
                        <Outlet />
                      </BootstrapGate>
                    </SetupGate>
                  }
                >
                  {publicRoutes.map((route) => (
                    <Route key={route.path} path={route.path} element={route.element} />
                  ))}
                  {bootstrapRoutes.map((route) => (
                    <Route key={route.path} path={route.path} element={route.element} />
                  ))}
                  {protectedStandaloneRoutes.map((route) => (
                    <Route
                      key={route.path}
                      path={route.path}
                      element={<ProtectedRoute>{route.element}</ProtectedRoute>}
                    />
                  ))}
                  <Route
                    element={
                      <ProtectedRoute>
                        <PrimaryRepositoryGate>
                          <WorkerProvider preload={["hash"]}>
                            <UploadProvider>
                              <AppShellLayout />
                            </UploadProvider>
                          </WorkerProvider>
                        </PrimaryRepositoryGate>
                      </ProtectedRoute>
                    }
                  >
                    {appRoutes.map((route) => (
                      <Route key={route.path} path={route.path} element={route.element} />
                    ))}
                  </Route>
                </Route>
              </Routes>
            </BrowserRouter>
          </AuthProvider>
          <HealthPoller />
        </QueryClientProvider>
        <Notifications />
      </GlobalProvider>
    </PreferencesEffects>
  );
}

export default App;
