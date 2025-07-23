import React, { useEffect } from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import SideBar from "@/components/SideBar";
import NavBar from "@/components/NavBar";
import { routes } from "@/routes/routes";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import GlobalProvider from "@/contexts/GlobalContext";
import "./App.css";
import Notifications from "@/components/Notifications";

const queryClient = new QueryClient();

function App(): React.ReactNode {
  const theme: string = localStorage.getItem("theme") || "light";

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

  return (
    <GlobalProvider>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <div className="flex flex-col h-screen">
            <div className="bg-base-100 shadow">
              <NavBar />
            </div>
            <div className="flex flex-1 overflow-hidden">
              <div className="w-auto bg-base-200 shadow-lg">
                <SideBar />
              </div>
              <div className="flex-1 p-4 overflow-y-auto">
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
                  © 2025 Lumilio Photos, Brought to you by EdwinZhan
                </p>
              </div>
            </footer>
          </div>
        </BrowserRouter>
      </QueryClientProvider>
      <Notifications />
    </GlobalProvider>
  );
}

export default App;
