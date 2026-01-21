import React, { useEffect } from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import SideBar from "@/components/SideBar";
import NavBar from "@/components/NavBar";
import { routes } from "@/routes/routes";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import GlobalProvider, { useGlobal } from "@/contexts/GlobalContext";
import "@/styles/App.css";
import "katex/dist/katex.min.css";
import Notifications from "@/components/Notifications";
import { SettingsProvider, useSettingsContext } from "./features/settings";
import { useI18n } from "@/lib/i18n.tsx";
import { pollHealth } from "@/services/healthService";
import { AuthProvider } from "./features/auth";

const queryClient = new QueryClient();

function HealthPoller(): React.ReactNode {
  const { state } = useSettingsContext();
  const { setOnline } = useGlobal();

  useEffect(() => {
    const intervalSec = state.server?.update_timespan ?? 5;
    const stop = pollHealth(intervalSec, ({ online }) => setOnline(online));
    return () => {
      stop();
    };
  }, [state.server?.update_timespan, setOnline]);

  return null;
}

function App(): React.ReactNode {
  const { t } = useI18n();

  return (
    <SettingsProvider>
      <GlobalProvider>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <BrowserRouter>
              <div className="flex flex-col h-screen">
                <div className="bg-base-100 shadow">
                  <NavBar />
                </div>
                <div className="flex flex-1 overflow-hidden">
                  <div className="w-auto bg-base-200 shadow-lg">
                    <SideBar />
                  </div>
                  <div className="flex-1 p-4 overflow-y-auto overflow-x-hidden">
                    <Routes>
                      {routes.map((route) => (
                        <Route
                          key={route.path}
                          path={route.path}
                          element={route.element}
                        />
                      ))}
                    </Routes>
                  </div>
                </div>
                <footer className="bg-base-100 text-base-content text-xs">
                  <div className="container mx-auto py-0.5">
                    <p className="text-center">
                      {t("footer.copyright", { year: new Date().getFullYear() })}
                    </p>
                  </div>
                </footer>
              </div>
            </BrowserRouter>
          </AuthProvider>
        </QueryClientProvider>
        <HealthPoller />
        <Notifications />
      </GlobalProvider>
    </SettingsProvider>
  );
}

export default App;
