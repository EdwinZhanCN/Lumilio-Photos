import { useCallback, useState } from "react";
import type { FileUploadProgress, PlannedFileUploadSession } from "./uploadProcessTypes.ts";

export function useUploadProgressState() {
  const [uploadProgress, setUploadProgress] = useState(0);
  const [fileProgress, setFileProgress] = useState<FileUploadProgress[]>([]);

  const initializeFileProgress = useCallback((sessions: PlannedFileUploadSession[]) => {
    setFileProgress(
      sessions.map((session) => ({
        fileName: session.file.name,
        progress: 0,
        status: "pending",
        sessionId: session.sessionId,
        isChunked: session.shouldUseChunks,
      })),
    );
  }, []);

  const updateFileProgress = useCallback(
    (sessionId: string, updates: Partial<FileUploadProgress>) => {
      setFileProgress((previous) =>
        previous.map((item) => (item.sessionId === sessionId ? { ...item, ...updates } : item)),
      );
    },
    [],
  );

  const reset = useCallback(() => {
    setUploadProgress(0);
    setFileProgress([]);
  }, []);

  return {
    uploadProgress,
    fileProgress,
    setUploadProgress,
    initializeFileProgress,
    updateFileProgress,
    reset,
  };
}
