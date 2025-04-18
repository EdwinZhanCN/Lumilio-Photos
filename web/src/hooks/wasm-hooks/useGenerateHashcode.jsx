import { useCallback, useRef } from 'react';
import {useMessage} from "@/hooks/util-hooks/useMessage.jsx";

/**
 * Custom hook for generating hashcode.
 * To use this hook, ensure that the Web Worker is initialized and ready to handle hash generation.
 * @author Edwin Zhan
 * @param {Object} options - The properties for the hook.
 * @param {Function} options.setIsGeneratingHashCodes - Function to set the is generating hashCode state.
 * @param {Function} options.setHashcodeProgress - Function to set the hashcode progress state.
 * @param {Function} options.onPerformanceMetrics - Function to handle performance metrics.
 * @param {Object} options.workerClientRef - Reference to the Web Worker client.
 * @param {boolean} options.wasmReady - Flag indicating if the WebAssembly module is ready.
 * @return {Object} - An object containing the generateHashCodes function.
 */
export const useGenerateHashcode = ({
    setIsGeneratingHashCodes,
    setHashcodeProgress,
    onPerformanceMetrics,
    workerClientRef,
    wasmReady,
}) => {
    const showMessage = useMessage();


    const perfMetricsRef = useRef({});

    /**
     * Generates hashcode for the given files, put the result by setHashResult.
     * And tracks the performance metrics.
     * @param {FileList} files - The files for which hashcode need to be generated.
     * @returns {[{index:number, hash:string}] | Error}
     */
    const generateHashCodes = useCallback(async (files) => {
        setIsGeneratingHashCodes(true);

        if (!workerClientRef.current || !wasmReady) {
            showMessage('error','WebAssembly module is not ready yet');
            setIsGeneratingHashCodes(false);
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
                numberProcessed: hashResults.length,
            };

            // Report performance metrics if callback is provided
            if (typeof onPerformanceMetrics === 'function') {
                onPerformanceMetrics(metrics);
            }

            if (!hashResults){
                showMessage('error','HashCode generation failed: No hash result');
                return new Error("HashCode generation failed: No hash result");
            }

            // The hash result if an array of objects, [{index:number,hash:string},...]
            return hashResults;
        } catch (error) {
            showMessage('error',`HashCode generation failed: ${error?.message || 'Unknown error'}`);
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
    }, [workerClientRef, wasmReady, setIsGeneratingHashCodes, setHashcodeProgress, onPerformanceMetrics]);

    return {
        generateHashCodes
    };
}