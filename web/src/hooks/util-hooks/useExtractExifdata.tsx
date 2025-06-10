import React, { useEffect } from "react";
import {useMessage} from "@/hooks/util-hooks/useMessage.tsx";
import {WorkerClient} from "@/workers/workerClient.ts";

type UseExtractExifdataProps = {
    workerClientRef: React.RefObject<WorkerClient | null>;
    setExtractExifProgress: React.Dispatch<React.SetStateAction<{
        numberProcessed: number;
        total: number;
        error?: string;
        failedAt?: number | null;
    } | null>>;
    setIsExtractingExif: (isExtracting: boolean) => void;
    setExifData: React.Dispatch<React.SetStateAction<Record<number, any> | null>>;
}

export const useExtractExifdata = ({
    workerClientRef,
    setExtractExifProgress,
    setIsExtractingExif,
    setExifData
}: UseExtractExifdataProps) => {
    const showMessage = useMessage();

    useEffect(() => {
        if (!workerClientRef.current) {
            return;
        }

        const progressListener = workerClientRef.current.addProgressListener((detail) => {
            if (detail && typeof detail.processed === 'number') {
                setExtractExifProgress((prev) => {
                    if (!prev) return prev;
                    return {
                        ...prev,
                        numberProcessed: detail.processed,
                        total: detail.total,
                    };
                });
            }
        });

        return () => {
            progressListener();
        };
    }, [workerClientRef, setExtractExifProgress]);

    /**
     * Extracts EXIF data from the given files.
     * @param {File[]} files - The files from which to extract EXIF data.
     */
    const extractExifData = async (files: File[]): Promise<void | Error> => {
        if (!workerClientRef.current) {
            showMessage('error', 'Worker client is not initialized');
            return new Error("Worker client is not initialized");
        }

        setIsExtractingExif(true);
        setExtractExifProgress({ numberProcessed: 0, total: files.length });

        try {
            const results = await workerClientRef.current.extractExif(files);

            if (results && results.exifResults) {
                const formattedExifData = results.exifResults.reduce((acc, item) => {
                    acc[item.index] = item.exifData;
                    return acc;
                }, {} as Record<number, any>);
                setExifData(formattedExifData);
            } else {
                setExifData(null);
            }
        } catch (error) {
            showMessage('error', `Failed to extract EXIF data: ${(error as Error).message}`);
            setExtractExifProgress(prev => ({
                ...prev!,
                error: (error as Error).message,
                failedAt: prev?.numberProcessed
            }));
            return error as Error;
        } finally {
            setIsExtractingExif(false);
        }
    };

    return { extractExifData };
}

