import { useEffect, useState, useRef } from "react";
import {
  Clock,
  CheckCircle,
  XCircle,
  AlertCircle,
  Loader2,
  Layers,
  Hash,
  Calendar,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  listJobs,
  getJobDuration,
  type JobDTO,
  type JobState,
} from "@/services/queueService";

// Format timestamp to relative time
function formatRelativeTime(timestamp: string): string {
  const date = new Date(timestamp);
  const now = new Date();
  const secondsAgo = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (secondsAgo < 60) return `${secondsAgo}s ago`;
  if (secondsAgo < 3600) return `${Math.floor(secondsAgo / 60)}m ago`;
  if (secondsAgo < 86400) return `${Math.floor(secondsAgo / 3600)}h ago`;
  if (secondsAgo < 604800) return `${Math.floor(secondsAgo / 86400)}d ago`;
  return date.toLocaleDateString();
}

// Get the most relevant timestamp based on job state
function getRelevantTimestamp(job: JobDTO): { label: string; time: string } {
  switch (job.state) {
    case "running":
      return {
        label: "Started",
        time: job.attempted_at || job.created_at || new Date().toISOString(),
      };
    case "completed":
    case "cancelled":
    case "discarded":
      return {
        label: "Finished",
        time: job.finalized_at || job.created_at || new Date().toISOString(),
      };
    case "retryable":
      return {
        label: "Failed",
        time: job.attempted_at || job.created_at || new Date().toISOString(),
      };
    case "scheduled":
      return {
        label: "Scheduled",
        time: job.scheduled_at || new Date().toISOString(),
      };
    case "available":
    default:
      return {
        label: "Created",
        time: job.created_at || new Date().toISOString(),
      };
  }
}

export function TaskMonitor() {
  const [jobs, setJobs] = useState<JobDTO[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterState, setFilterState] = useState<JobState | "all">("all");
  const [filterQueue, setFilterQueue] = useState<string>("all");
  const [timeRange, setTimeRange] = useState<"1h" | "24h" | "30d" | "all">(
    "24h",
  );
  const [currentCursor, setCurrentCursor] = useState<string | undefined>(
    undefined,
  );
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [pageHistory, setPageHistory] = useState<(string | undefined)[]>([]);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageInput, setPageInput] = useState("1");
  const [totalCount, setTotalCount] = useState<number | null>(null);
  const [totalPages, setTotalPages] = useState<number | null>(null);

  const parentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let mounted = true;

    const fetchJobs = async () => {
      try {
        const params: any = {
          limit: 50,
          include_count: currentPage === 1, // Only fetch count on first page
        };

        if (filterState !== "all") {
          params.state = filterState;
        }

        if (filterQueue !== "all") {
          params.queue = filterQueue;
        }

        if (timeRange !== "all") {
          params.time_range = timeRange;
        }

        if (currentCursor) {
          params.cursor = currentCursor;
        }

        const data = await listJobs(params);
        if (mounted && data) {
          setJobs(data.jobs ?? []);
          setNextCursor(data.cursor);
          setError(null);

          // Update total count and calculate total pages
          if (data.total_count !== undefined) {
            setTotalCount(data.total_count);
            setTotalPages(Math.ceil(data.total_count / 50));
          }
        }
      } catch (err) {
        if (mounted) {
          setError("Failed to fetch jobs");
          console.error("Job list error:", err);
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };

    // Initial fetch
    fetchJobs();

    // Poll every 5 seconds (slower since we have pagination now)
    const interval = setInterval(fetchJobs, 5000);

    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, [filterState, filterQueue, timeRange, currentCursor]);

  // Get unique queues from jobs for filter
  const queues = Array.from(new Set(jobs.map((j) => j.queue)));

  // Virtual scrolling
  const rowVirtualizer = useVirtualizer({
    count: jobs.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 80,
    overscan: 5,
  });

  // Pagination handlers
  const handleNextPage = () => {
    if (nextCursor) {
      setPageHistory([...pageHistory, currentCursor]);
      setCurrentCursor(nextCursor);
      setCurrentPage(currentPage + 1);
      setPageInput(String(currentPage + 1));
    }
  };

  const handlePrevPage = () => {
    if (pageHistory.length > 0) {
      const newHistory = [...pageHistory];
      const prevCursor = newHistory.pop();
      setPageHistory(newHistory);
      setCurrentCursor(prevCursor);
      setCurrentPage(currentPage - 1);
      setPageInput(String(currentPage - 1));
    }
  };

  const handlePageInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setPageInput(e.target.value);
  };

  const handlePageJump = () => {
    const targetPage = parseInt(pageInput);
    if (isNaN(targetPage) || targetPage < 1) {
      setPageInput(String(currentPage));
      return;
    }

    if (targetPage === currentPage) return;

    // Reset to first page and navigate
    if (targetPage === 1) {
      setCurrentCursor(undefined);
      setPageHistory([]);
      setCurrentPage(1);
      setPageInput("1");
    } else {
      // For other pages, user needs to navigate manually
      // This is a limitation of cursor-based pagination
      setPageInput(String(currentPage));
    }
  };

  const handleTimeRangeChange = (range: "1h" | "24h" | "30d" | "all") => {
    setTimeRange(range);
    // Reset pagination when changing filters
    setCurrentCursor(undefined);
    setNextCursor(undefined);
    setPageHistory([]);
    setCurrentPage(1);
    setPageInput("1");
    setTotalCount(null);
    setTotalPages(null);
  };

  // Get state icon
  const getStateIcon = (state: JobState) => {
    switch (state) {
      case "completed":
        return <CheckCircle className="w-4 h-4" />;
      case "running":
        return <Loader2 className="w-4 h-4 animate-spin" />;
      case "available":
      case "scheduled":
        return <Clock className="w-4 h-4" />;
      case "retryable":
        return <AlertCircle className="w-4 h-4" />;
      case "cancelled":
      case "discarded":
        return <XCircle className="w-4 h-4" />;
      default:
        return <Clock className="w-4 h-4" />;
    }
  };

  // Get state color
  const getStateBadgeClass = (state: JobState): string => {
    switch (state) {
      case "completed":
        return "badge-success";
      case "running":
        return "badge-info";
      case "available":
      case "scheduled":
        return "badge-ghost";
      case "retryable":
        return "badge-warning";
      case "cancelled":
      case "discarded":
        return "badge-error";
      default:
        return "badge-ghost";
    }
  };

  if (loading) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <Loader2 className="w-8 h-8 animate-spin mx-auto text-primary" />
        <p className="mt-3 text-sm opacity-60">Loading jobs...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <AlertCircle className="w-8 h-8 mx-auto text-error" />
        <div className="text-error mt-2">{error}</div>
      </div>
    );
  }

  return (
    <div className="bg-base-100 rounded-lg shadow-sm h-full flex flex-col overflow-hidden">
      {/* Filter Controls - Sticky Header */}
      <div className="sticky top-0 z-10 bg-base-100 p-3 border-b border-base-300 space-y-2">
        {/* First Row: Filters */}
        <div className="flex flex-wrap gap-2 items-center">
          <div className="flex items-center gap-2">
            <label className="text-xs opacity-60 uppercase tracking-wide font-semibold">
              Time:
            </label>
            <div className="btn-group">
              <button
                className={`btn btn-xs ${timeRange === "1h" ? "btn-active" : ""}`}
                onClick={() => handleTimeRangeChange("1h")}
              >
                &lt; 1h
              </button>
              <button
                className={`btn btn-xs ${timeRange === "24h" ? "btn-active" : ""}`}
                onClick={() => handleTimeRangeChange("24h")}
              >
                &lt; 1d
              </button>
              <button
                className={`btn btn-xs ${timeRange === "30d" ? "btn-active" : ""}`}
                onClick={() => handleTimeRangeChange("30d")}
              >
                &lt; 30d
              </button>
              <button
                className={`btn btn-xs ${timeRange === "all" ? "btn-active" : ""}`}
                onClick={() => handleTimeRangeChange("all")}
              >
                All
              </button>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <label className="text-xs opacity-60 uppercase tracking-wide font-semibold">
              State:
            </label>
            <select
              className="select select-xs select-bordered"
              value={filterState}
              onChange={(e) => {
                setFilterState(e.target.value as JobState | "all");
                setCurrentCursor(undefined);
                setPageHistory([]);
                setCurrentPage(1);
                setPageInput("1");
                setTotalCount(null);
                setTotalPages(null);
              }}
            >
              <option value="all">All</option>
              <option value="running">Running</option>
              <option value="available">Available</option>
              <option value="scheduled">Scheduled</option>
              <option value="retryable">Retryable</option>
              <option value="completed">Completed</option>
              <option value="cancelled">Cancelled</option>
              <option value="discarded">Discarded</option>
            </select>
          </div>

          <div className="flex items-center gap-2">
            <label className="text-xs opacity-60 uppercase tracking-wide font-semibold">
              Queue:
            </label>
            <select
              className="select select-xs select-bordered"
              value={filterQueue}
              onChange={(e) => {
                setFilterQueue(e.target.value);
                setCurrentCursor(undefined);
                setPageHistory([]);
                setCurrentPage(1);
                setPageInput("1");
                setTotalCount(null);
                setTotalPages(null);
              }}
            >
              <option value="all">All</option>
              {queues.map((q) => (
                <option key={q} value={q}>
                  {q}
                </option>
              ))}
            </select>
          </div>

          <div className="ml-auto text-xs opacity-60 flex items-center gap-1">
            <Hash className="w-3 h-3" />
            {jobs.length}
          </div>
        </div>

        {/* Second Row: Pagination */}
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <button
              className="btn btn-xs btn-circle btn-ghost"
              onClick={handlePrevPage}
              disabled={pageHistory.length === 0}
            >
              <ChevronLeft className="w-4 h-4" />
            </button>

            <div className="flex items-center gap-1">
              <input
                type="text"
                className="input input-xs input-bordered w-12 text-center"
                value={pageInput}
                onChange={handlePageInputChange}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    handlePageJump();
                  }
                }}
                onBlur={handlePageJump}
              />
              <span className="text-xs opacity-60">
                {totalPages !== null ? `/ ${totalPages}` : ""}
              </span>
            </div>

            <button
              className="btn btn-xs btn-circle btn-ghost"
              onClick={handleNextPage}
              disabled={!nextCursor || jobs.length === 0}
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          <div className="text-xs opacity-60">
            {totalCount !== null ? (
              <span>{totalCount} total jobs</span>
            ) : nextCursor ? (
              <span>More results available</span>
            ) : null}
          </div>
        </div>
      </div>

      {/* Job List - Scrollable with Virtual Scrolling */}
      <div ref={parentRef} className="flex-1 overflow-y-auto">
        {jobs.length === 0 ? (
          <div className="p-8 text-center opacity-60">
            <Clock className="w-12 h-12 mx-auto mb-2 opacity-30" />
            <p className="text-sm">No jobs found with current filters</p>
          </div>
        ) : (
          <ul
            className="list relative"
            style={{
              height: `${rowVirtualizer.getTotalSize()}px`,
            }}
          >
            {rowVirtualizer.getVirtualItems().map((virtualRow) => {
              const job = jobs[virtualRow.index];
              const duration = getJobDuration(job);
              const hasErrors = job.errors && job.errors.length > 0;
              const badgeClass = getStateBadgeClass((job.state || 'available') as JobState);
              const timestamp = getRelevantTimestamp(job);
              const relativeTime = formatRelativeTime(timestamp.time);

              return (
                <li
                  key={job.id}
                  className="list-row hover:bg-base-200/50 transition-colors absolute top-0 left-0 w-full"
                  style={{
                    height: `${virtualRow.size}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  {/* State Badge with Icon */}
                  <div
                    className={`badge ${badgeClass} badge-sm flex-shrink-0 font-mono flex items-center`}
                  >
                    {getStateIcon((job.state || 'available') as JobState)}
                    <span className="hidden sm:inline sm:ml-1">
                      {job.state}
                    </span>
                  </div>

                  {/* Job Info - grows to fill space */}
                  <div className="list-col-grow min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-semibold text-sm truncate">
                        {job.kind}
                      </span>
                      <span className="text-xs opacity-50 flex items-center gap-1">
                        <Hash className="w-3 h-3" />
                        {job.id}
                      </span>
                    </div>

                    {/* Metadata Row */}
                    <div className="flex items-center gap-2 mt-1 text-xs opacity-60 flex-wrap">
                      <span className="flex items-center gap-1">
                        <Layers className="w-3 h-3" />
                        {job.queue}
                      </span>

                      <span>•</span>
                      <span className="flex items-center gap-1">
                        <Calendar className="w-3 h-3" />
                        <span title={new Date(timestamp.time).toLocaleString()}>
                          {relativeTime}
                        </span>
                      </span>

                      {duration && (
                        <>
                          <span>•</span>
                          <span className="flex items-center gap-1">
                            <Clock className="w-3 h-3" />
                            {duration}
                          </span>
                        </>
                      )}

                      {(job.attempt ?? 0) > 0 && (
                        <>
                          <span>•</span>
                          <span>
                            {job.attempt ?? 0}/{job.max_attempts ?? 0} attempts
                          </span>
                        </>
                      )}

                      {job.priority !== 1 && (
                        <>
                          <span>•</span>
                          <span className="badge badge-ghost badge-xs">
                            P{job.priority}
                          </span>
                        </>
                      )}
                    </div>

                    {/* Error Message */}
                    {hasErrors && (
                      <div className="flex items-start gap-1 text-xs text-error mt-2 bg-error/10 rounded px-2 py-1">
                        <AlertCircle className="w-3 h-3 flex-shrink-0 mt-0.5" />
                        <span className="line-clamp-2">{job.errors![0]}</span>
                      </div>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}
