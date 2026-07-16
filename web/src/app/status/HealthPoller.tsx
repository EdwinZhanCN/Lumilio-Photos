import { useEffect, type ReactNode } from "react";
import { useGlobal } from "@/contexts/GlobalContext";
import { usePreference } from "@/features/settings";
import { $api } from "@/lib/http-commons/queryClient";

/** Keeps the global connectivity indicator synchronized with the health endpoint. */
export default function HealthPoller(): ReactNode {
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
