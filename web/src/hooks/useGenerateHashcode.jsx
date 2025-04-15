import { useCallback, useRef } from 'react';

/**
 * Custom hook for generating hashcode.
 * To use this hook, ensure that the Web Worker is initialized and ready to handle hash generation.
 * @author Edwin Zhan
 * @param {Object} props - The properties for the hook.
 * @param {Object} props.workerClientRef - The worker client reference.
 * @param {boolean} props.wasmReady - Flag indicating if WebAssembly is ready.
 * @param {Function} props.setError - Function to set the error state.
 * @param {Function} props.setIsGeneratingHashCodes - Function to set the is generating hashCode state.
 * @param {Function} props.setHashcodeProgress - Function to set the hashcode progress state.
 * @param {Function} props.onPerformanceMetrics - Function to handle performance metrics.
 * @returns {[{index:number, hash:string}] | Error}
 */
export const useGenerateHashcode = ({
    workerClientRef,
    wasmReady,
    setError,
    setIsGeneratingHashCodes,
    setHashcodeProgress,
    onPerformanceMetrics,
}) => {
    const perfMetricsRef = useRef({});

    /**
     * Generates hashcode for the given files, put the result by setHashResult.
     * And tracks the performance metrics.
     * @param {FileList} files - The files for which hashcode need to be generated.
     */
    const generateHashCodes = useCallback(async (files) => {
        setIsGeneratingHashCodes(true);
        setError(null);

        if (!workerClientRef.current || !wasmReady) {
            setError('WebAssembly module is not ready yet');
            setTimeout(() => setError(''), 3000);
            return new Error("WebAssembly module is not ready yet");
        }

        // Initialize performance metrics
        perfMetricsRef.current = {
            startTime: performance.now(),
            fileCount: files.length,
            totalSize: Array.from(files).reduce((sum, file) => sum + file.size, 0),
            processingTime: 0
        };

        const removeProgressListener = workerClientRef.current.addProgressListener(({ processed }) => {
            setHashcodeProgress(prev => ({
                ...prev,
                numberProcessed: processed,
                total: files.length,
            }));
        });

        try {
            // The hash result if an array of objects, [{index:number,hash:string},...]
            const { hashResults } = await workerClientRef.current.generateHash(Array.from(files));

            // Calculate performance metrics
            const endTime = performance.now();
            const processingTime = endTime - perfMetricsRef.current.startTime;

            const metrics = {
                ...perfMetricsRef.current,
                processingTime,
                filesPerSecond: files.length / (processingTime / 1000),
                bytesPerSecond: perfMetricsRef.current.totalSize / (processingTime / 1000),
            };

            // Report performance metrics if callback is provided
            if (typeof onPerformanceMetrics === 'function') {
                onPerformanceMetrics(metrics);
            }

            if (!hashResults){
                setError('HashCode generation failed: No hash result');
                setTimeout(() => setError(''), 3000);
                return new Error("HashCode generation failed: No hash result");
            }

            // The hash result if an array of objects, [{index:number,hash:string},...]
            return hashResults;
        } catch (error) {
            setError(`HashCode generation failed: ${error?.message || 'Unknown error'}`);
            setHashcodeProgress(prev => ({
                ...prev,
                error: error?.message,
                failedAt: Date.now(),
            }));
        } finally {
            setIsGeneratingHashCodes(false);
            removeProgressListener();
            setHashcodeProgress(null);
        }
    }, [workerClientRef, wasmReady, setError, setIsGeneratingHashCodes, setHashcodeProgress, onPerformanceMetrics]);

    return {
        generateHashCodes
    };
}