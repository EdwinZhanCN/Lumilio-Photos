import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import GlobalProvider from "@/contexts/GlobalContext";
import "@/styles/App.css";
import "katex/dist/katex.min.css";
import "streamdown/styles.css";
import { Notifications } from "@/features/notifications";
import { PreferencesEffects } from "@/features/settings";
import { AuthProvider } from "@/features/auth";
import AppRouter from "@/app/router/AppRouter";
import HealthPoller from "@/app/status/HealthPoller";

const queryClient = new QueryClient();

function App(): ReactNode {
  return (
    <PreferencesEffects>
      <GlobalProvider>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <AppRouter />
          </AuthProvider>
          <HealthPoller />
        </QueryClientProvider>
        <Notifications />
      </GlobalProvider>
    </PreferencesEffects>
  );
}

export default App;
