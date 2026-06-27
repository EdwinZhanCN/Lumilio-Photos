import React, { useEffect } from "react";
import {
  BrowserRouter,
  Outlet,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import SideBar from "@/components/SideBar";
import NavBar from "@/components/NavBar";
import { ChatDock } from "@/features/lumilio/components/Chat/ChatDock";
import {
  appRoutes,
  bootstrapRoutes,
  protectedStandaloneRoutes,
  publicRoutes,
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
import { BreadcrumbProvider, Breadcrumbs } from "@/components/breadcrumbs";

const queryClient = new QueryClient();

function AppShellLayout(): React.ReactNode {
  const { t } = useI18n();
  const location = useLocation();
  // The /lumilio board embeds its own dock; everywhere else gets the global FAB.
  const showAgentDock = location.pathname !== "/lumilio";

  return (
    <BreadcrumbProvider>
      <div className="flex h-screen flex-col">
        <div className="bg-base-100 shadow">
          <NavBar />
        </div>
        <div className="flex flex-1 overflow-hidden">
          <div className="w-auto bg-base-200 shadow-lg">
            <SideBar />
          </div>
          <div id="app-scroll-container" className="flex-1 overflow-y-auto overflow-x-hidden">
            <Breadcrumbs className="sticky top-0 z-10 bg-base-100/80 backdrop-blur" />
            <Outlet />
          </div>
        </div>
        <footer className="bg-base-100 text-base-content text-xs">
          <div className="container mx-auto py-0.5">
            <p className="text-center">
              {t("footer.copyright", {
                year: new Date().getFullYear(),
              })}
            </p>
          </div>
        </footer>
        {showAgentDock && <ChatDock variant="fab" />}
      </div>
    </BreadcrumbProvider>
  );
}

function HealthPoller(): React.ReactNode {
  const [healthCheckIntervalMs] = usePreference("healthCheckIntervalMs");
  const { setOnline } = useGlobal();

  const intervalMs = Math.max(
    1000,
    Math.min(50_000, Math.max(1000, healthCheckIntervalMs)),
  );

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
              <SetupGate>
                <BootstrapGate>
                  <Routes>
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
                  </Routes>
                </BootstrapGate>
              </SetupGate>
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
