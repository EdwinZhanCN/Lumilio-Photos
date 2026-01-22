import {useState, useCallback, useEffect, useRef} from "react";
import { useWorker } from "@/contexts/WorkerProvider.tsx";
import { SingleHashResult } from "@/workers/workerClient";

export interface HashcodeProgress {
  numberProcessed: number;
  total: number;
  error?: string;
}


interface HashcodeResult {
  hash: string;
  index: number;
}

export interface useGenerateHashcodeReturn {
  isGenerating: boolean;
  progress: HashcodeProgress | null;
  generateHashCodes: (
    files: FileList | File[],
    onChunkProcessed?: (result: HashcodeResult) => void,
  ) => Promise<HashcodeResult[] | undefined>;
  cancelGeneration: () => void;
}

/**
 * Custom hook for generating file hashcodes using a Web Worker.
 * Manages its own state for progress and generation status.
 * This hook must be used within a component tree wrapped by `<WorkerProvider />`.
 * @author Edwin Zhan
 * @since 1.1.0
 * @returns {useGenerateHashcodeReturn} Hook state and actions for hashcode generation.
 */
export const useGenerateHashcode = (): useGenerateHashcodeReturn => {
  const workerClient = useWorker();
  const [isGenerating, setIsGenerating] = useState(false);

  // 使用 Ref 存储最新进度，不触发渲染
  const progressRef = useRef({ processed: 0, total: 0 });
  // 用于 UI 渲染的 State，仅在 rAF 中更新
  const [displayProgress, setDisplayProgress] = useState({ processed: 0, total: 0 });

  // 节流渲染循环
  useEffect(() => {
    if (!isGenerating) return;

    let animationFrameId: number;

    const tick = () => {
      setDisplayProgress(prev => {
        // 只有数据变了才更新 state，避免无效渲染
        if (prev.processed !== progressRef.current.processed) {
          return { ...progressRef.current };
        }
        return prev;
      });
      animationFrameId = requestAnimationFrame(tick);
    };

    tick(); // 启动循环

    return () => cancelAnimationFrame(animationFrameId);
  }, [isGenerating]);

  const generateHashCodes = useCallback(async (
    files: FileList | File[],
    onChunkProcessed?: (result: HashcodeResult) => void
  ): Promise<HashcodeResult[] | undefined> => {
    setIsGenerating(true);
    const filesArray = Array.isArray(files) ? files : Array.from(files);
    progressRef.current = { processed: 0, total: filesArray.length };
    const results: HashcodeResult[] = [];

    try {
      await workerClient.generateHash(filesArray, (result: SingleHashResult) => {
        // 1. 更新 Ref (不触发渲染)
        progressRef.current.processed++;

        const hashResult = { hash: result.hash, index: result.index };
        results.push(hashResult);

        // 2. 立即把结果扔出去，给上传队列 (流水线核心)
        if (onChunkProcessed) {
          onChunkProcessed(hashResult);
        }
      });
      return results;
    } catch (error) {
      console.error("Batch processing failed", error);
      return undefined;
    } finally {
      setIsGenerating(false);
    }
  }, [workerClient]);

  const cancelGeneration = useCallback(() => {
    workerClient.abortGenerateHash();
    setIsGenerating(false);
  }, [workerClient]);

  return {
    isGenerating,
    progress: {
      numberProcessed: displayProgress.processed,
      total: displayProgress.total
    },
    generateHashCodes,
    cancelGeneration
  };
};
