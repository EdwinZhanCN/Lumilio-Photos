import { Suspense, type ReactNode } from "react";
import { BrowserRouter, Outlet, Route, Routes } from "react-router-dom";
import { BootstrapGate, PrimaryRepositoryGate, ProtectedRoute, SetupGate } from "@/features/auth";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { UploadProvider } from "@/features/upload";
import AppShellLayout from "@/app/shell/AppShellLayout";
import NotFound from "@/app/router/NotFound";
import RouteLoadingFallback from "@/app/router/RouteLoadingFallback";
import {
  appRoutes,
  bootstrapRoutes,
  protectedStandaloneRoutes,
  publicRoutes,
  shareRoutes,
} from "@/app/router/routes";

/** Owns route registration and the gates/providers attached to route groups. */
export default function AppRouter(): ReactNode {
  return (
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
              <Route
                key={route.path}
                path={route.path}
                element={<Suspense fallback={<RouteLoadingFallback />}>{route.element}</Suspense>}
              />
            ))}
          </Route>
        </Route>
        {/* Keep the catch-all outside setup and authentication gates so an
            unknown URL always explains itself instead of redirecting. */}
        <Route path="*" element={<NotFound />} />
      </Routes>
    </BrowserRouter>
  );
}
