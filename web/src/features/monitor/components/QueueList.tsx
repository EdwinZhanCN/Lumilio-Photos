import { useEffect, useState } from "react";
import { Clock, Zap, Loader2, AlertCircle } from "lucide-react";
import { listQueues, type QueueStatsDTO } from "@/services/queueService";

export function QueueList() {
  const [queues, setQueues] = useState<QueueStatsDTO[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;

    const fetchQueues = async () => {
      try {
        const data = await listQueues();
        if (mounted) {
          setQueues(data?.queues ?? []);
          setError(null);
        }
      } catch (err) {
        if (mounted) {
          setError("Failed to fetch queues");
          console.error("Queue list error:", err);
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };

    // Initial fetch
    fetchQueues();

    // Poll every 10 seconds
    const interval = setInterval(fetchQueues, 10000);

    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  if (loading) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <Loader2 className="w-8 h-8 animate-spin mx-auto text-primary" />
        <p className="mt-3 text-sm opacity-60">Loading queues...</p>
      </div>
    );
  }

  if (error || queues.length === 0) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <AlertCircle className="w-8 h-8 mx-auto text-warning" />
        <div className="text-warning mt-2 text-sm">
          {error || "No active queues found"}
        </div>
      </div>
    );
  }

  return (
    <div className="bg-base-100 rounded-lg shadow-sm h-full flex flex-col overflow-hidden">
      {/* Header - Sticky */}
      <div className="sticky top-0 z-10 bg-base-100 p-3 border-b border-base-300">
        <div className="flex items-center justify-between">
          <span className="text-sm font-semibold opacity-60">
            {queues.length} Active Queue{queues.length !== 1 ? "s" : ""}
          </span>
        </div>
      </div>

      {/* Queue List - Scrollable with DaisyUI List */}
      <div className="flex-1 overflow-y-auto">
        <ul className="list">
          {queues.map((queue) => {
            const lastUpdate = queue.updated_at ? new Date(queue.updated_at) : new Date();
            const now = new Date();
            const secondsAgo = Math.floor(
              (now.getTime() - lastUpdate.getTime()) / 1000,
            );

            let timeAgo = "";
            if (secondsAgo < 60) {
              timeAgo = `${secondsAgo}s ago`;
            } else if (secondsAgo < 3600) {
              timeAgo = `${Math.floor(secondsAgo / 60)}m ago`;
            } else {
              timeAgo = `${Math.floor(secondsAgo / 3600)}h ago`;
            }

            // Determine if queue is stale (no update in 5+ minutes)
            const isStale = secondsAgo > 300;
            const isVeryActive = secondsAgo < 30;

            return (
              <li
                key={queue.name}
                className={`list-row hover:bg-base-200/50 transition-all duration-200 ${isStale ? "opacity-60" : ""
                  }`}
              >
                {/* Queue Icon */}
                <div
                  className={`flex-shrink-0 ${isVeryActive ? "text-success" : "text-primary"
                    }`}
                >
                  <Zap className="w-5 h-5" />
                </div>

                {/* Queue Name & Info - grows to fill space */}
                <div className="list-col-grow min-w-0">
                  <h4 className="font-semibold text-sm truncate">
                    {queue.name}
                  </h4>
                  {/* Last Activity */}
                  <div className="flex items-center gap-1 text-xs opacity-60 mt-1">
                    <Clock className="w-3 h-3" />
                    <span>{timeAgo}</span>
                  </div>
                </div>

                {/* Status Badge */}
                {isStale ? (
                  <div className="badge badge-ghost badge-xs gap-1 flex-shrink-0">
                    Idle
                  </div>
                ) : (
                  <div className="badge badge-success badge-xs gap-1 flex-shrink-0 animate-pulse">
                    Active
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}
