import {
    useState,
    RefObject
} from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { formatBytes } from '@/utils/formatters'; // Example utility
import { useMessage } from '@/hooks/util-hooks/useMessage'; // Example custom hook
import { useGenerateHashcode } from '@/hooks/wasm-hooks/useGenerateHashcode'; // Example custom hook
import { uploadService } from '@/services/uploadService'; // Example service

/**
 * A single file processing result indicating a failed file with name and error.
 */
interface FailedFile {
    name: string;
    error: string;
}

/**
 * The shape of the processFiles result, containing arrays of file names that were
 * duplicates, successfully uploaded, or failed.
 */
interface ProcessResults {
    duplicates: string[];
    uploaded: string[];
    failed: FailedFile[];
}

/**
 * The return type of the processFiles function.
 * (Adjust "files" parameter type if you only accept File[] instead of FileList.)
 */
type ProcessFilesFn = (
    files: FileList | File[]
) => Promise<ProcessResults>;

/**
 * useUploadProcess is a custom hook that handles the upload process of files.
 * @param {React.RefObject} workerClientRef - Reference to your WASM worker client
 * @param {boolean} wasmReady - Indicates if WASM is ready
 * @returns {{
 *   processFiles: (function(*): Promise<{duplicates: *[], uploaded: *[], failed: *[]}>),
 *   isUploading: boolean,
 *   isChecking: boolean,
 *   resetStatus: Function,
 *   uploadProgress: number
 * }}
 */
export function useUploadProcess(
    workerClientRef: RefObject<any>,
    wasmReady: boolean
): {
    processFiles: ProcessFilesFn;
    isUploading: boolean;
    isChecking: boolean;
    resetStatus: () => void;
    uploadProgress: number;
    hashcodeProgress: {
        numberProcessed?: number | undefined;
        total?: number | undefined;
        error?: string | undefined;
        failedAt?: number | undefined;
    } | null;
    isGeneratingHashCodes: boolean;
} {
    const queryClient = useQueryClient();

    const [uploadProgress, setUploadProgress] = useState<number>(0);
    const [hashcodeProgress, setHashcodeProgress] = useState<{
        numberProcessed?: number | undefined;
        total?: number | undefined;
        error?: string | undefined;
        failedAt?: number | undefined;
    } | null>(null);
    const [isGeneratingHashCodes, setIsGeneratingHashCodes] = useState<boolean>(false);
    const showMessage = useMessage();

    const handlePerformanceMetrics = (metrics: {
        numberProcessed: number;
        totalSize: number;
        processingTime: number;
        bytesPerSecond: number;
    }) => {
        const formattedSpeed = formatBytes(metrics.bytesPerSecond) + '/s';
        const timeInSeconds = (metrics.processingTime / 1000).toFixed(2);
        const formattedSize = formatBytes(metrics.totalSize);

        console.log(
            'hint',
            `Processed ${metrics.numberProcessed} files (${formattedSize}) in ${timeInSeconds}s at ${formattedSpeed}`
        );
    };

    const { generateHashCodes } = useGenerateHashcode({
        setIsGeneratingHashCodes,
        setHashcodeProgress,
        onPerformanceMetrics: handlePerformanceMetrics,
        workerClientRef,
        wasmReady
    });

    // Bloom filter batch check
    const bloomFilterCheck = useMutation({
        mutationFn: (hashes: string[]) => uploadService.batchCheckHashes(hashes),
        onSuccess: () => {
            return queryClient.invalidateQueries({ queryKey: ['bloomFilterResults'] });
        }
    });

    // Database verification
    const verifyInDatabase = useMutation({
        mutationFn: (hash: string) => uploadService.verifyHashInDatabase(hash)
    });

    // File uploading
    interface UploadArgs {
        file: File;
        hash: string;
        onUploadProgress?: (progressEvent: ProgressEvent) => void;
    }

    // You can further refine the mutation types here if needed
    const uploadFile = useMutation({
        mutationFn: async ({ file, hash }: UploadArgs) => {
            return await uploadService.uploadFile(file, hash, {
                onUploadProgress(progressEvent) {
                    const progress = progressEvent.total ? Math.round((progressEvent.loaded * 100) / progressEvent.total) : 0;
                    console.log("Upload progress:", progress);
                },
            });
        },
        onSuccess: () => {
            return queryClient.invalidateQueries({ queryKey: ['userAssets'] });
        },
        onSettled: () => {
            // Reset progress when upload completes
            setUploadProgress(0);
        }
    });

    /**
     * Process files according to the sequence diagram
     */
    const processFiles: ProcessFilesFn = async (files) => {
        const results: ProcessResults = {
            duplicates: [],
            uploaded: [],
            failed: []
        };

        try {
            const startTime = performance.now();

            // 1. Generate hashes using the WASM worker
            const hashResults = await generateHashCodes(files);

            /**
             * Match each original file with its generated hash.
             * This array might be typed as an object containing file + hash fields.
             */
            const filesWithHash = Array.from(files).map((file, index) => {
                if (hashResults instanceof Error) {
                    return new Error(hashResults.message);
                }
                const hashResult = hashResults?.find((result: any) => result.index === index);
                const endTime = performance.now();
                showMessage(
                    'info',
                    `Processed in ${((endTime - startTime) / 1000).toFixed(2)} seconds`
                );
                return {
                    file,
                    hash: hashResult?.hash as string
                };
            });

            console.log('filesWithHash', filesWithHash); // TODO: Remove

            // (Your Bloom filter check logic goes here, if desired.)
            // Example usage (commented out):
            /*
            const hashes = filesWithHash.map(item => item.hash);
            const bloomResult = await bloomFilterCheck.mutateAsync(hashes);

            for (let i = 0; i < filesWithHash.length; i++) {
              const { file, hash } = filesWithHash[i];

              if (!bloomResult.data[i]) {
                // Hash definitely doesn't exist, upload
                try {
                  await uploadFile.mutateAsync({ file, hash });
                  results.uploaded.push(file.name);
                } catch (error: any) {
                  results.failed.push({ name: file.name, error: error.message });
                }
              } else {
                // Potential match, verify in DB
                try {
                  const dbResult = await verifyInDatabase.mutateAsync(hash);
                  if (dbResult.data.exists) {
                    results.duplicates.push(file.name);
                  } else {
                    await uploadFile.mutateAsync({ file, hash });
                    results.uploaded.push(file.name);
                  }
                } catch (error: any) {
                  results.failed.push({ name: file.name, error: error.message });
                }
              }
            }
            */
        } catch (error) {
            console.error('Upload process failed:', error);
        }

        return results;
    };

    return {
        processFiles,
        isUploading: uploadFile.isPending, // For React Query, adjust to .isLoading or .isPending based on version
        isChecking: bloomFilterCheck.isPending || verifyInDatabase.isPending,
        resetStatus: () => {
            bloomFilterCheck.reset();
            verifyInDatabase.reset();
            uploadFile.reset();
            setUploadProgress(0);
        },
        uploadProgress,
        hashcodeProgress,
        isGeneratingHashCodes
    };
}