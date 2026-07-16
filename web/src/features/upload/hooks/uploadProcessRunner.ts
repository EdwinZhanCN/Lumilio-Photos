import {
  generateSessionId,
  getResumableSessionId,
  shouldUseChunks,
} from "@/lib/upload/uploadTransport";
import { getOptimalBatchSize, ProcessingPriority } from "@/lib/utils/smartBatchSizing.ts";
import { createUploadTransport } from "./uploadProcessTransport.ts";
import type {
  FileUploadSession,
  PlannedFileUploadSession,
  UploadProcessMessages,
  UploadProgressCallbacks,
  UploadRunResult,
  UploadTransportConfig,
} from "./uploadProcessTypes.ts";
import type { UploadTransportDependencies } from "./uploadProcessTransport.ts";

export interface UploadHashResult {
  hash: string;
  index: number;
}

export interface UploadProcessRunnerDependencies
  extends
    UploadProgressCallbacks,
    Omit<UploadTransportDependencies, keyof UploadProgressCallbacks> {
  messages: UploadProcessMessages;
  generateHashCodes: (
    files: FileList | File[],
    onHashReady: (result: UploadHashResult) => void,
  ) => Promise<unknown>;
  initializeFileProgress: (sessions: PlannedFileUploadSession[]) => void;
  config: UploadTransportConfig;
}

const getErrorMessage = (error: unknown, fallback: string): string =>
  error instanceof Error ? error.message : fallback;

export const runUploadProcess = async (
  files: FileList | File[],
  dependencies: UploadProcessRunnerDependencies,
): Promise<UploadRunResult> => {
  const fileArray = Array.from(files);
  if (fileArray.length === 0) return { results: [], resultSessions: new Map() };

  const plannedSessions: PlannedFileUploadSession[] = fileArray.map((file) => {
    const chunked = shouldUseChunks(file);
    return {
      file,
      sessionId: chunked
        ? getResumableSessionId(file, dependencies.repositoryId)
        : generateSessionId(),
      shouldUseChunks: chunked,
    };
  });
  dependencies.initializeFileProgress(plannedSessions);

  const transport = createUploadTransport(dependencies);
  const uploadTasks: Promise<void>[] = [];
  const smallFileBuffer: FileUploadSession[] = [];
  const optimalBatchSize = getOptimalBatchSize(
    "thumbnail",
    fileArray.length,
    ProcessingPriority.CRITICAL,
  );
  const startTime = performance.now();
  let totalBytesHashed = 0;

  try {
    await dependencies.generateHashCodes(files, (hashResult) => {
      const session = plannedSessions[hashResult.index];
      if (!session) return;
      totalBytesHashed += session.file.size;

      const sessionWithHash: FileUploadSession = {
        ...session,
        hash: hashResult.hash,
      };
      if (sessionWithHash.shouldUseChunks) {
        uploadTasks.push(transport.uploadChunked(sessionWithHash));
        return;
      }

      smallFileBuffer.push(sessionWithHash);
      if (smallFileBuffer.length >= optimalBatchSize) {
        uploadTasks.push(transport.uploadBatch([...smallFileBuffer]));
        smallFileBuffer.length = 0;
      }
    });

    const durationSeconds = Math.max((performance.now() - startTime) / 1000, Number.EPSILON);
    const sizeInMegabytes = totalBytesHashed / (1024 * 1024);
    console.log(
      `[Hash Metrics] Processed ${fileArray.length} files (${sizeInMegabytes.toFixed(2)} MB) in ${durationSeconds.toFixed(2)}s. Average speed: ${(sizeInMegabytes / durationSeconds).toFixed(2)} MB/s`,
    );

    if (smallFileBuffer.length > 0) {
      uploadTasks.push(transport.uploadBatch([...smallFileBuffer]));
    }

    await Promise.all(uploadTasks);
    await transport.waitForMaterialization();
    return transport.getResult();
  } catch (error) {
    await Promise.allSettled(uploadTasks);
    const message = getErrorMessage(error, dependencies.messages.processFailed);
    plannedSessions.forEach((session) =>
      dependencies.updateFileProgress(session.sessionId, {
        status: "failed",
        error: message,
      }),
    );
    throw error;
  }
};
