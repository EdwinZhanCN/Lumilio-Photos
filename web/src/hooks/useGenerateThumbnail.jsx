import { useCallback } from 'react';
import { useUpload } from '@/contexts/UploadContext';

const BATCH_SIZE = 10; // TODO: add this in system config

/**
 * Custom hook to generate thumbnails using a Web Worker.
 *
 * @author Edwin Zhan
 * @param {Object} options - Configuration options
 * @param {Function} options.setGenThumbnailProgress - Function to set the progress of thumbnail generation
 * @param {Function} options.setIsGenThumbnails - Function to set the state of thumbnail generation
 * @returns {Object} Object containing the generatePreviews function
 */
export const useGenerateThumbnail = ({setGenThumbnailProgress, setIsGenThumbnails}) => {
    // General State
    const {
        setError,
        setPreviews,
        workerClientRef,
        wasmReady,
    } = useUpload();

    /**
     * Generates thumbnails for the given files.
     * @param {File[]} files - The files for which thumbnails need to be generated.
     */
    const generatePreviews = useCallback(async (files) => {
        if (!workerClientRef.current || !wasmReady) {
            setError('WebAssembly module is not ready yet');
            return;
        }

        const removeProgressListener = workerClientRef.current.addProgressListener(({ processed }) => {
            setGenThumbnailProgress(prev => ({
                ...prev,
                numberProcessed: processed,
                total: files.length,
            }));
        });

        try {
            setIsGenThumbnails(true);
            const startIndex = 0;
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
            setGenThumbnailProgress(prev => ({
                ...prev,
                error: error?.message,
                failedAt: Date.now(),
            }));
        } finally {
            setIsGenThumbnails(false);
            removeProgressListener();
            setGenThumbnailProgress(null);
        }
    }, [wasmReady, workerClientRef, setError, setPreviews, setIsGenThumbnails, setGenThumbnailProgress]);

    return { generatePreviews };
};