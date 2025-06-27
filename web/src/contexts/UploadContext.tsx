import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  ReactNode,
  DragEvent,
  RefObject,
  useReducer,
  useMemo,
  Dispatch,
} from "react";
import { WasmWorkerClient } from "@/workers/workerClient";
import { useUploadProcess } from "@/hooks/api-hooks/useUploadProcess";
import { useMessage } from "@/hooks/util-hooks/useMessage";

// 1. 定义 Reducer 的 State, Action 和初始状态

interface UploadState {
  files: File[];
  previews: (string | null)[];
  filesCount: number;
  isDragging: boolean;
  wasmReady: boolean;
  readonly maxPreviewFiles: number;
}

// 使用联合类型，为 Action 提供类型安全
type UploadAction =
  | { type: "SET_DRAGGING"; payload: boolean }
  | {
      type: "SET_FILES";
      payload: { files: File[]; previews: (string | null)[] };
    }
  | { type: "SET_WASM_READY"; payload: boolean }
  | { type: "CLEAR_FILES" };

const initialState: UploadState = {
  files: [],
  previews: [],
  filesCount: 0,
  isDragging: false,
  wasmReady: false,
  maxPreviewFiles: 30, // 只读值，可以放在 initial state
};

// 2. 创建 Reducer 函数来处理所有状态逻辑
const uploadReducer = (
  state: UploadState,
  action: UploadAction,
): UploadState => {
  switch (action.type) {
    case "SET_DRAGGING":
      return { ...state, isDragging: action.payload };
    case "SET_FILES":
      // 撤销旧的预览 URL 防止内存泄漏
      state.previews.forEach((url) => url && URL.revokeObjectURL(url));
      return {
        ...state,
        files: action.payload.files,
        previews: action.payload.previews,
        filesCount: action.payload.files.length,
      };
    case "SET_WASM_READY":
      return { ...state, wasmReady: action.payload };
    case "CLEAR_FILES":
      // 撤销旧的预览 URL
      state.previews.forEach((url) => url && URL.revokeObjectURL(url));
      return { ...state, files: [], previews: [], filesCount: 0 };
    default:
      return state;
  }
};

// 3. 定义 Context 的值类型 (不再暴露 set 方法)
interface UploadContextValue {
  state: UploadState;
  dispatch: Dispatch<UploadAction>; // 暴露 dispatch 以便在组件中有更大的灵活性
  workerClientRef: React.RefObject<WasmWorkerClient | null>;
  handleDragOver: (e: DragEvent) => void;
  handleDragLeave: (e: DragEvent) => void;
  handleDrop: (e: DragEvent, handleFiles?: (files: FileList) => void) => void;
  clearFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;
  BatchUpload: (selectedFiles: FileList) => Promise<void>;
  isProcessing: boolean;
  resetUploadStatus: () => void;
  uploadProgress: number;
  hashcodeProgress: {
    numberProcessed?: number;
    total?: number;
    error?: string;
    failedAt?: number;
  } | null;
  isGeneratingHashCodes: boolean;
}

interface UploadProviderProps {
  children: ReactNode;
}

export const UploadContext = createContext<UploadContextValue | undefined>(
  undefined,
);

export default function UploadProvider({ children }: UploadProviderProps) {
  // 4. 使用 useReducer 替换多个 useState
  const [state, dispatch] = useReducer(uploadReducer, initialState);
  const { wasmReady, previews } = state; // 从 state 中解构

  const showMessage = useMessage();
  const workerClientRef = useRef<WasmWorkerClient | null>(null);
  const uploadProcess = useUploadProcess(workerClientRef, wasmReady);

  // 5. 事件处理器现在 dispatch actions
  const handleDragOver = useCallback((e: DragEvent) => {
    e.preventDefault();
    dispatch({ type: "SET_DRAGGING", payload: true });
  }, []);

  const handleDragLeave = useCallback((e: DragEvent) => {
    e.preventDefault();
    dispatch({ type: "SET_DRAGGING", payload: false });
  }, []);

  const handleDrop = useCallback(
    (e: DragEvent, handleFiles?: (files: FileList) => void) => {
      e.preventDefault();
      dispatch({ type: "SET_DRAGGING", payload: false });
      const droppedFiles = e.dataTransfer?.files;
      if (handleFiles && droppedFiles?.length) {
        handleFiles(droppedFiles); // 外部组件的 handleFiles 应该调用 dispatch
      }
    },
    [],
  );

  const clearFiles = useCallback(
    (fileInputRef: RefObject<HTMLInputElement | null>) => {
      dispatch({ type: "CLEAR_FILES" });
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
    [],
  );

  useEffect(() => {
    if (!workerClientRef.current) {
      workerClientRef.current = new WasmWorkerClient();
    }
    const initWasm = async () => {
      try {
        await workerClientRef.current?.initGenThumbnailWASM();
        await workerClientRef.current?.initGenHashWASM();
        dispatch({ type: "SET_WASM_READY", payload: true });
        console.log("WASM module initialized successfully");
      } catch (error) {
        console.error("Failed to initialize WASM:", error);
      }
    };
    initWasm();

    // 组件卸载时，清理最后的预览
    return () => {
      previews.forEach((url) => url && URL.revokeObjectURL(url));
    };
  }, [previews]); // previews 作为依赖项，确保卸载时使用的是最新的预览列表

  const BatchUpload = useCallback(
    async (selectedFiles: FileList) => {
      if (!wasmReady || !selectedFiles.length) {
        showMessage(
          "error",
          "Cannot upload: WASM not initialized or no files selected",
        );
        return;
      }
      try {
        await uploadProcess.processFiles(selectedFiles);
      } catch (error: any) {
        showMessage("error", `Upload process failed: ${error.message}`);
      }
      uploadProcess.resetStatus();
    },
    [wasmReady, uploadProcess, showMessage],
  );

  // 6. 使用 useMemo 包装 contextValue 以进行性能优化
  const contextValue = useMemo(
    () => ({
      state,
      dispatch, // 暴露 dispatch
      workerClientRef,
      handleDragOver,
      handleDragLeave,
      handleDrop,
      clearFiles,
      BatchUpload,
      isProcessing:
        uploadProcess.isGeneratingHashCodes || uploadProcess.isUploading,
      resetUploadStatus: uploadProcess.resetStatus,
      uploadProgress: uploadProcess.uploadProgress,
      hashcodeProgress: uploadProcess.hashcodeProgress,
      isGeneratingHashCodes: uploadProcess.isGeneratingHashCodes,
    }),
    [
      state,
      handleDragOver,
      handleDragLeave,
      handleDrop,
      clearFiles,
      BatchUpload,
      uploadProcess.isGeneratingHashCodes,
      uploadProcess.isUploading,
      uploadProcess.resetStatus,
      uploadProcess.uploadProgress,
      uploadProcess.hashcodeProgress,
    ],
  );

  return (
    <UploadContext.Provider value={contextValue}>
      {children}
    </UploadContext.Provider>
  );
}

// 7. useUploadContext 保持不变，但现在返回的是新的 context 值
export function useUploadContext() {
  const context = useContext(UploadContext);
  if (!context) {
    throw new Error("useUploadContext must be used within an UploadProvider");
  }
  return context;
}
