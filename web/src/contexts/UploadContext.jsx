import {createContext, useCallback, useContext, useEffect, useRef, useState} from 'react';
import {WasmWorkerClient} from "@/workers/workerClient.js";

// UploadContext is a context for uploadAssets page
export const UploadContext = createContext();

/**
 * UploadProvider is a provider component for the upload assets page
 *
 * It provides a context with the following states:
 * - files: Array of selected files
 * - previews: Array of file preview URLs
 * - error: Error message string
 * - success: Success message string
 * - maxPreviewFiles: Maximum number of files that can be previewed
 * - isDragging: Boolean flag indicating if a drag event is in progress
 * - handleDragOver: Function to handle drag over event
 * - handleDragLeave: Function to handle drag leave event
 * - handleDrop: Function to handle drop event, takes a handleFiles function as an argument
 *
 * @author Edwin Zhan
 * @param {React.ReactNode} children - Child components that will have access to the context
 * @returns {JSX.Element} - The provider component
 */
export default function UploadProvider({ children }) {
    // General states
    const [files, setFiles] = useState([]);
    const [previews, setPreviews] = useState([]);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const [maxPreviewFiles] = useState(30);
    const [isDragging, setIsDragging] = useState(false);
    const [wasmReady, setWasmReady] = useState(false);
    const [hint, setHint] = useState('');

    // Worker client reference
    const workerClientRef = useRef(null);

    const handleDragOver = useCallback((e) => {
        e.preventDefault();
        setIsDragging(true);
    }, []);

    const handleDragLeave = useCallback((e) => {
        e.preventDefault();
        setIsDragging(false);
    }, []);

    const handleDrop = useCallback((e, handleFiles) => {
        e.preventDefault();
        setIsDragging(false);
        const droppedFiles = e.dataTransfer.files;
        if (handleFiles) {
            handleFiles(droppedFiles);
        }
    }, []);


    // Clean up generated URLs
    const revokePreviews = useCallback((urls) => {
        urls.forEach(url => {
            if (url) URL.revokeObjectURL(url);
        });
    }, []);

    /**
     * Clears the selected files and generated previews.
     * @param {React.RefObject} fileInputRef - Reference to the file input element
     */
    const clearFiles = (fileInputRef) => {
        revokePreviews(previews);
        setFiles([]);
        setPreviews([]);

        // Reset file input values
        if (fileInputRef.current) {
            fileInputRef.current.value = "";
        }
    };


    useEffect(() => {
        if (!workerClientRef.current) {
            workerClientRef.current = new WasmWorkerClient();
        }

        // Initialize WASM
        const initWasm = async () => {
            try {
                await workerClientRef.current.initGenThumbnailWASM();
                await workerClientRef.current.initGenHashWASM();
                setWasmReady(true);
                console.log('WASM module initialized successfully');
            } catch (error) {
                console.error('Failed to initialize WASM:', error);
                setError('Failed to initialize WebAssembly module');
            }
        };

        initWasm();

        // Cleanup worker when component unmounts
        return () => {
            if (workerClientRef.current) {
                workerClientRef.current.terminateGenerateThumbnailWorker();
                workerClientRef.current.terminateGenerateHashWorker();
            }
            revokePreviews(previews);
        };
    }, []);


    return (
        <UploadContext.Provider value={{
            files,
            setFiles,
            previews,
            setPreviews,
            error,
            setError,
            success,
            setSuccess,
            maxPreviewFiles,
            isDragging,
            handleDragOver,
            handleDragLeave,
            handleDrop,
            wasmReady,
            workerClientRef,
            clearFiles,
            hint,
            setHint,
        }}>
            <div className="min-h-screen px-2">
                {children}
            </div>
        </UploadContext.Provider>
    );
}

export const useUpload = () => useContext(UploadContext);