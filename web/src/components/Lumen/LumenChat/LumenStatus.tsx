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
    <div className="p-2 bg-info text-info-content">
      <span className="loading loading-spinner loading-sm my-2" />
      Loading {progress.initStatus}
    </div>
  );
}
