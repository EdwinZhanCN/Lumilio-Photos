import { ServerStackIcon } from "@heroicons/react/24/outline";
import { useSettingsContext } from "@/features/settings";

export default function ServerSettings() {
  const { state, dispatch } = useSettingsContext();
  const value = state.server.update_timespan;

  // Reasonable presets within [1, 50] seconds
  const presets = [1, 2, 5, 10, 30, 50];

  const setTimespan = (v: number) => {
    const clamped = Math.min(50, Math.max(1, v));
    dispatch({ type: "SET_SERVER_UPDATE_TIMESPAN", payload: clamped });
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-2">
        <ServerStackIcon className="size-6 text-primary" />
        <h2 className="text-2xl font-bold">Server</h2>
      </div>

      <section className="space-y-2">
        <h3 className="text-lg font-semibold">Health check interval</h3>
        <p className="text-sm opacity-70">
          Control how often the app pings the server for health status. Lower
          values update more frequently but may increase network usage.
        </p>
        <div className="flex items-center gap-3">
          <input
            type="range"
            min={1}
            max={50}
            step={0.5}
            value={value}
            className="range range-primary"
            onChange={(e) => setTimespan(Number(e.target.value))}
          />
          <div className="min-w-24 text-right font-mono tabular-nums">
            {value.toFixed(1)}s
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm opacity-70 mr-1">Presets:</span>
          {presets.map((p) => (
            <button
              key={p}
              className={`btn btn-xs sm:btn-sm ${value === p ? "btn-primary" : "btn-outline"}`}
              onClick={() => setTimespan(p)}
            >
              {p}s
            </button>
          ))}
          <div className="divider divider-horizontal mx-1" />
          <button
            className="btn btn-xs sm:btn-sm btn-ghost"
            onClick={() => setTimespan(5)}
            title="Reset to default (5s)"
          >
            Reset
          </button>
        </div>
      </section>

      <div className="alert alert-info">
        <span>
          The health check interval determines how frequently the server status
          (online/offline) is refreshed throughout the app.
        </span>
      </div>
    </div>
  );
}
