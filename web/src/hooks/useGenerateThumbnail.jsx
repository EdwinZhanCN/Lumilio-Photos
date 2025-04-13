// hooks/useGenerateThumbnail.js
import { useCallback, useRef } from 'react';

const BATCH_SIZE = 10; // TODO: add this in system config

/**
 * Custom hook to generate thumbnails using a Web Worker.
 * To use this hook, ensure that the Web Worker is initialized and ready to handle thumbnail generation.
 * @author Edwin Zhan
 * @param workerClientRef - A reference to the Web Worker client.
 * @param wasmReady - A boolean indicating if the WebAssembly module is ready.
 * @param setError - A function to set error messages.
 * @param setPreviews - A function to set the generated thumbnail previews.
 * @param setIsGeneratingThumbnails - A function to set the thumbnail generation status.
 * @param setThumbnailProgress - A function to set the thumbnail generation progress.
 * @returns {{generatePreviews: ((function(*): Promise<void>)|*)}}
 */
export const useGenerateThumbnail = ({
    workerClientRef,
    wasmReady,
    setError,
    setPreviews,
    setIsGeneratingThumbnails,
    setThumbnailProgress,
}) => {
    /**
     * Generates thumbnails for the given files.
     * @param files - The files for which thumbnails need to be generated.
     */
    const generatePreviews = useCallback(async (files) => {
        if (!workerClientRef.current || !wasmReady) {
            setError('WebAssembly module is not ready yet');
            return;
        }

        const removeProgressListener = workerClientRef.current.addProgressListener(({ processed }) => {
            setThumbnailProgress(prev => ({
                ...prev,
                numberProcessed: processed,
                total: files.length,
            }));
        });

        try {
            setIsGeneratingThumbnails(true);
            const startIndex = 0; // 根据实际逻辑调整
            const fileArray = Array.from(files);

            for (let i = 0; i < fileArray.length; i += BATCH_SIZE) {
                const batch = fileArray.slice(i, i + BATCH_SIZE);
                const result = await workerClientRef.current.generateThumbnail({
                    files: batch,
                    batchIndex: i / BATCH_SIZE,
                    startIndex: startIndex + i,
                });

                if (result.status === 'complete' && result.results) {
                    setPreviews(prev => {
                        const newPreviews = [...prev];
                        result.results.forEach(({ index, url }) => {
                            const actualIndex = startIndex + index;
                            if (url && actualIndex < newPreviews.length) {
                                newPreviews[actualIndex] = url;
                            }
                        });
                        return newPreviews;
                    });
                }
            }
        } catch (error) {
            setError(`Thumbnail generation failed: ${error?.message || 'Unknown error'}`);
            setThumbnailProgress(prev => ({
                ...prev,
                error: error?.message,
                failedAt: Date.now(),
            }));
        } finally {
            setIsGeneratingThumbnails(false);
            removeProgressListener();
            setThumbnailProgress(null);
        }
    }, [wasmReady, workerClientRef, setError, setPreviews, setIsGeneratingThumbnails, setThumbnailProgress]);

    return { generatePreviews };
};