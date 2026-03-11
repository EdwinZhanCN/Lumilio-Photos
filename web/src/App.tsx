import React, { useEffect } from "react";
import { BrowserRouter, Outlet, Route, Routes } from "react-router-dom";
import SideBar from "@/components/SideBar";
import NavBar from "@/components/NavBar";
import { appRoutes, publicRoutes } from "@/routes/routes";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import GlobalProvider, { useGlobal } from "@/contexts/GlobalContext";
import "@/styles/App.css";
import "katex/dist/katex.min.css";
import Notifications from "@/components/Notifications";
import { SettingsProvider, useSettingsContext } from "./features/settings";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import { AuthProvider, ProtectedRoute } from "./features/auth";

const queryClient = new QueryClient();

function AppShellLayout(): React.ReactNode {
  const { t } = useI18n();

  return (
    <div className="flex h-screen flex-col">
      <div className="bg-base-100 shadow">
        <NavBar />
      </div>
      <div className="flex flex-1 overflow-hidden">
        <div className="w-auto bg-base-200 shadow-lg">
          <SideBar />
        </div>
        <div
          id="app-scroll-container"
          className="flex-1 overflow-y-auto overflow-x-hidden p-4"
        >
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
    </div>
  );
}

function HealthPoller(): React.ReactNode {
  const { state } = useSettingsContext();
  const { setOnline } = useGlobal();

  const intervalSec = state.server?.update_timespan ?? 5;
  const intervalMs = Math.max(
    1000,
    Math.min(50, Math.max(1, intervalSec)) * 1000,
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
    <SettingsProvider>
      <GlobalProvider>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <BrowserRouter>
              <Routes>
                {publicRoutes.map((route) => (
                  <Route key={route.path} path={route.path} element={route.element} />
                ))}
                <Route
                  element={
                    <ProtectedRoute>
                      <AppShellLayout />
                    </ProtectedRoute>
                  }
                >
                  {appRoutes.map((route) => (
                    <Route key={route.path} path={route.path} element={route.element} />
                  ))}
                </Route>
              </Routes>
            </BrowserRouter>
          </AuthProvider>
          <HealthPoller />
        </QueryClientProvider>
        <Notifications />
      </GlobalProvider>
    </SettingsProvider>
  );
}

export default App;
