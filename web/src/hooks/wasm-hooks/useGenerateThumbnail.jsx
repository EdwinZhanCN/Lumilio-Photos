import { useCallback } from 'react';
import {useMessage} from "@/hooks/util-hooks/useMessage.jsx";

const BATCH_SIZE = 10; // TODO: add this in system config

/**
 * Custom hook to generate thumbnails using a Web Worker.
 *
 * @author Edwin Zhan
 * @param {Object} options - Configuration options
 * @param {Function} options.setGenThumbnailProgress - Function to set the progress of thumbnail generation
 * @param {Function} options.setIsGenThumbnails - Function to set the state of thumbnail generation
 * @param {Object} options.workerClientRef - Reference to the Web Worker client
 * @param {boolean} options.wasmReady - Flag indicating if the WebAssembly module is ready
 * @param {Function} options.setPreviews - Function to set the generated thumbnail previews
 * @returns {Object} Object containing the generatePreviews function
 */
export const useGenerateThumbnail = ({setGenThumbnailProgress, setIsGenThumbnails, workerClientRef, wasmReady, setPreviews}) => {
    const showMessage = useMessage();

    /**
     * Generates thumbnails for the given files.
     * @param {File[]} files - The files for which thumbnails need to be generated.
     */
    const generatePreviews = useCallback(async (files) => {
        if (!workerClientRef.current || !wasmReady) {
            showMessage('error','WebAssembly module is not ready yet');
            return new Error("WebAssembly module is not ready yet");
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

            for (let i = 0; i < files.length; i += BATCH_SIZE) {
                /**
                 * @type {FileList}
                 */
                const batch = files.slice(i, i + BATCH_SIZE);
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
                        if (!newPreviews){
                            showMessage('error','Thumbnail generation failed: No previews generated');
                            return new Error("Thumbnail generation failed");
                        }
                        return newPreviews;
                    });
                }
            }
        } catch (error) {
            showMessage('error',`Thumbnail generation failed: ${error?.message || 'Unknown error'}`);
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
    }, [wasmReady, workerClientRef, setPreviews, setIsGenThumbnails, setGenThumbnailProgress]);

    return { generatePreviews };
};