import React, {
    createContext,
    useCallback,
    useContext,
    useEffect,
    useRef,
    useState,
    ReactNode,
    DragEvent,
    RefObject
} from 'react';
import { WasmWorkerClient } from '@/workers/workerClient';
import { useUploadProcess } from '@/hooks/api-hooks/useUploadProcess';
import { useMessage } from '@/hooks/util-hooks/useMessage';

interface UploadContextValue {
    files: File[];
    setFiles: React.Dispatch<React.SetStateAction<File[]>>;
    filesCount: number;
    setFilesCount: React.Dispatch<React.SetStateAction<number>>;
    previews: (string | null)[];
    setPreviews: React.Dispatch<React.SetStateAction<(string | null)[]>>;
    maxPreviewFiles: number;
    isDragging: boolean;
    handleDragOver: (e: DragEvent) => void;
    handleDragLeave: (e: DragEvent) => void;
    handleDrop: (
        e: DragEvent,
        handleFiles?: (files: FileList) => void
    ) => void;
    wasmReady: boolean;
    workerClientRef: React.RefObject<WasmWorkerClient | null>;
    clearFiles: (fileInputRef: RefObject<HTMLInputElement | null>) => void;
    BatchUpload: (selectedFiles: FileList) => Promise<void>;
    isProcessing: boolean;
    resetUploadStatus: () => void;
    uploadProgress: number;
    hashcodeProgress: {
        numberProcessed?: number | undefined;
        total?: number | undefined;
        error?: string | undefined;
        failedAt?: number | undefined;
    } | null;
    isGeneratingHashCodes: boolean;
}

interface UploadProviderProps {
    children: ReactNode;
}

export const UploadContext = createContext<UploadContextValue | undefined>(
    undefined
);

/**
 * UploadProvider is a provider component for the upload assets page
 *
 * It provides a context with the following states and methods
 */
export default function UploadProvider({ children }: UploadProviderProps) {
    // General states
    const [files, setFiles] = useState<File[]>([]);
    const [previews, setPreviews] = useState<(string | null)[]>([]);
    const [maxPreviewFiles] = useState<number>(30);
    const [isDragging, setIsDragging] = useState<boolean>(false);
    const [wasmReady, setWasmReady] = useState<boolean>(false);
    const [filesCount, setFilesCount] = useState<number>(0);

    const showMessage = useMessage();

    // Worker client reference
    const workerClientRef = useRef<WasmWorkerClient | null>(null);

    const handleDragOver = useCallback((e: DragEvent) => {
        e.preventDefault();
        setIsDragging(true);
    }, []);

    const handleDragLeave = useCallback((e: DragEvent) => {
        e.preventDefault();
        setIsDragging(false);
    }, []);

    const handleDrop = useCallback(
        (e: DragEvent, handleFiles?: (files: FileList) => void) => {
            e.preventDefault();
            setIsDragging(false);
            const droppedFiles = e.dataTransfer?.files;
            if (handleFiles && droppedFiles?.length) {
                handleFiles(droppedFiles);
            }
        },
        []
    );

    // Clean up generated URLs
    const revokePreviews = useCallback((urls: (string | null)[]) => {
        urls.forEach((url) => {
            if (url) {
                URL.revokeObjectURL(url);
            }
        });
    }, []);

    /**
     * Clears the selected files and generated previews.
     */
    const clearFiles = (fileInputRef: RefObject<HTMLInputElement|null>) => {
        revokePreviews(previews);
        setFiles([]);
        setPreviews([]);

        // Reset file input values
        if (fileInputRef.current) {
            fileInputRef.current.value = '';
        }
    };

    useEffect(() => {
        if (!workerClientRef.current) {
            workerClientRef.current = new WasmWorkerClient();
        }

        const initWasm = async () => {
            try {
                await workerClientRef.current?.initGenThumbnailWASM();
                await workerClientRef.current?.initGenHashWASM();
                setWasmReady(true);
                console.log('WASM module initialized successfully');
            } catch (error) {
                console.error('Failed to initialize WASM:', error);
            }
        };

        initWasm();

        // Cleanup worker when component unmounts
        return () => {
            if (workerClientRef.current) {
                // workerClientRef.current.terminateGenerateThumbnailWorker();
                // workerClientRef.current.terminateGenerateHashWorker();
            }
            revokePreviews(previews);
        };
    }, [revokePreviews, previews]);

    // Upload process usage
    const uploadProcess = useUploadProcess(workerClientRef, wasmReady);

    /**
     * BatchUpload function to handle batch upload of files.
     */
    const BatchUpload = useCallback(
        async (selectedFiles: FileList) => {
            if (!wasmReady || !selectedFiles.length) {
                showMessage('error', 'Cannot upload: WASM not initialized or no files selected');
                return;
            }
            try {
                const results = await uploadProcess.processFiles(selectedFiles);

                showMessage('success', `Successfully uploaded ${results.uploaded.length} files`);

                if (results.duplicates.length) {
                    showMessage('hint', `${results.duplicates.length} duplicate files were skipped`);
                }

                if (results.failed.length) {
                    showMessage('error', `Failed to upload ${results.failed.length} files`);
                }
            } catch (error: any) {
                showMessage('error', `Upload process failed: ${error.message}`);
            }
        },
        [wasmReady, uploadProcess, showMessage]
    );

    const contextValue: UploadContextValue = {
        files,
        setFiles,
        previews,
        setPreviews,
        maxPreviewFiles,
        isDragging,
        handleDragOver,
        handleDragLeave,
        handleDrop,
        wasmReady,
        workerClientRef,
        clearFiles,
        BatchUpload,
        filesCount,
        setFilesCount,
        isProcessing: uploadProcess.isChecking || uploadProcess.isUploading,
        resetUploadStatus: uploadProcess.resetStatus,
        uploadProgress: uploadProcess.uploadProgress,
        hashcodeProgress: uploadProcess.hashcodeProgress,
        isGeneratingHashCodes: uploadProcess.isGeneratingHashCodes
    };

    return (
        <UploadContext.Provider value={contextValue}>
            <div className="min-h-screen px-2">{children}</div>
        </UploadContext.Provider>
    );
}

export function useUploadContext() {
    const context = useContext(UploadContext);
    if (!context) {
        throw new Error('useUploadContext must be used within an UploadProvider');
    }
    return context;
}