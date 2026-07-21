import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { render } from "vitest-browser-react";
import { I18nProvider } from "@/lib/i18n";
import GlobalProvider from "@/contexts/GlobalContext";
import { AuthProvider } from "@/features/auth/state/AuthProvider";

type Options = {
  /** Initial history entry for the enclosing router. */
  route?: string;
  /** Wrap in a router. Off for components that must work without one. */
  router?: boolean;
  /**
   * Wrap in the real AuthProvider for flows that consume `useAuth`. On mount
   * with empty storage it settles to idle without any request; specs that
   * complete a session must serve `/api/v1/auth/media-token` via MSW.
   */
  auth?: boolean;
};

/**
 * Renders a component inside the app's real providers — i18n, global context,
 * a fresh QueryClient and a router — so tests exercise the real internal chain
 * and mock only at the HTTP boundary through MSW. Retries are disabled so a
 * mocked error surfaces immediately instead of being retried. Returns the
 * vitest-browser-react render result; await it, as render is async.
 */
export function renderWithProviders(
  ui: ReactNode,
  { route = "/", router = true, auth = false }: Options = {},
) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  const routed = router ? <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter> : ui;
  const authed = auth ? (
    <AuthProvider resetFeatureState={() => {}}>{routed}</AuthProvider>
  ) : (
    routed
  );

  return render(
    <I18nProvider>
      <GlobalProvider>
        <QueryClientProvider client={queryClient}>{authed}</QueryClientProvider>
      </GlobalProvider>
    </I18nProvider>,
  );
}
