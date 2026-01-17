interface LumenStatusProps {
  isInitializing: boolean;
  progress?: {
    initStatus?: string;
  } | null;
}

export function LumenStatus({ isInitializing, progress }: LumenStatusProps) {
  if (!isInitializing || !progress || !progress.initStatus) {
    return null;
  }

  return (
    <div className="mx-4 mt-4 mb-2 p-3 rounded-xl bg-gradient-to-r from-blue-50 to-purple-50 border border-blue-200 shadow-sm">
      <div className="flex items-center gap-2">
        <div className="relative">
          <span className="loading loading-spinner loading-sm text-blue-600"></span>
          <span className="absolute inset-0 loading loading-spinner loading-sm text-purple-600 opacity-60 animate-pulse"></span>
        </div>
        <span className="text-sm font-medium text-blue-700">
          Initializing {progress.initStatus}
        </span>
      </div>
      <div className="w-full bg-blue-100 rounded-full h-1.5 mt-2 overflow-hidden">
        <div
          className="bg-gradient-to-r from-blue-500 to-purple-600 h-1.5 rounded-full animate-pulse"
          style={{ width: "70%" }}
        ></div>
      </div>
    </div>
  );
}

export default LumenStatus;
