// src/hooks/useUploadProcess.js
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {uploadService} from '@/services/uploadService';
import {useGenerateHashcode} from "@/hooks/wasm-hooks/useGenerateHashcode.jsx";
import {formatBytes} from "@/utils/formatters.js";
import {useState} from "react";
import {useMessage} from "@/hooks/util-hooks/useMessage.jsx";

/**
 * useUploadProcess is a custom hook that handles the upload process of files.
 * @param {React.RefObject} workerClientRef
 * @param {boolean} wasmReady
 * @returns {{processFiles: (function(*): Promise<{duplicates: *[], uploaded: *[], failed: *[]}>), isUploading: boolean, isChecking: boolean, resetStatus: *, uploadProgress: number}}
 */
export function useUploadProcess(workerClientRef, wasmReady) {
    const queryClient = useQueryClient();
    const [uploadProgress, setUploadProgress] = useState(0);
    const [hashcodeProgress, setHashcodeProgress] = useState(null);
    const [isGeneratingHashCodes, setIsGeneratingHashCodes] = useState(false);
    const showMessage = useMessage();

    const handlePerformanceMetrics = (metrics) => {
        const formattedSpeed = formatBytes(metrics.bytesPerSecond) + '/s';
        const timeInSeconds = (metrics.processingTime / 1000).toFixed(2);
        const formattedSize = formatBytes(metrics.totalSize);

        console.log('hint',`Processed ${metrics.numberProcessed} files (${formattedSize}) in ${timeInSeconds}s at ${formattedSpeed}`);
    };

    const { generateHashCodes } = useGenerateHashcode({
        setIsGeneratingHashCodes: setIsGeneratingHashCodes,
        setHashcodeProgress: setHashcodeProgress,
        onPerformanceMetrics: handlePerformanceMetrics,
        workerClientRef: workerClientRef,
        wasmReady: wasmReady,
    });


    // Bloom filter batch check
    const bloomFilterCheck = useMutation({
        mutationFn: (hashes) => uploadService.batchCheckHashes(hashes),
        onSuccess: () => {
            return queryClient.invalidateQueries({ queryKey: ['bloomFilterResults'] });
        },
    });

    // Database verification
    const verifyInDatabase = useMutation({
        mutationFn: (hash) => uploadService.verifyHashInDatabase(hash)
    });

    // File upload
    const uploadFile = useMutation({
        mutationFn: async ({ file, hash }) => {
            return await uploadService.uploadFile(file, hash, {
                onUploadProgress: (progressEvent) => {
                    const progress = Math.round(
                        (progressEvent.loaded * 100) / progressEvent.total
                    );
                    setUploadProgress(progress);
                }
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

    // Process files according to the sequence diagram
    const processFiles = async (files) => {

        const results = {
            duplicates: [],
            uploaded: [],
            failed: []
        };

        try {
            const startTime = performance.now();
            // 1. Generate hashes using the WASM worker
            const hashResults = await generateHashCodes(files);

            /**
             *
             * @type {{file: File, hash: string}[]}
             */
            const filesWithHash = Array.from(files).map((file, index) => {
                const hashResult = hashResults.find(result => result.index === index);
                const endTime = performance.now();
                showMessage('info',`Processed in ${((endTime - startTime) / 1000).toFixed(2)} seconds`);
                return {
                    file,
                    hash: hashResult?.hash
                };
            });

            // TODO: Uncomment the following code when the bloom filter is ready
            // // 2. Check all hashes against the bloom filter
            const hashes = filesWithHash.map(item => item.hash);
            // const bloomResult = await bloomFilterCheck.mutateAsync(hashes);
            //
            // // 3. Process based on bloom filter results
            // for (let i = 0; i < filesWithHash.length; i++) {
            //     const { file, hash } = filesWithHash[i];
            //
            //     if (!bloomResult.data[i]) {
            //         // Hash definitely doesn't exist, upload directly
            //         try {
            //             await uploadFile.mutateAsync({
            //                 file,
            //                 hash,
            //             });
            //             results.uploaded.push(file.name);
            //         } catch (error) {
            //             results.failed.push({ name: file.name, error: error.message });
            //         }
            //     } else {
            //         // Potential match, verify in database
            //         try {
            //             const dbResult = await verifyInDatabase.mutateAsync(hash);
            //
            //             if (dbResult.data.exists) {
            //                 results.duplicates.push(file.name);
            //             } else {
            //                 // False positive in bloom filter, upload
            //                 await uploadFile.mutateAsync({
            //                     file,
            //                     hash,
            //                 });
            //                 results.uploaded.push(file.name);
            //             }
            //         } catch (error) {
            //             results.failed.push({ name: file.name, error: error.message });
            //         }
            //     }
            // }
        } catch (error) {
            console.error("Upload process failed:", error);
        }

        return results;
    };

    return {
        processFiles,
        isUploading: uploadFile.isPending,
        isChecking: bloomFilterCheck.isPending || verifyInDatabase.isPending,
        resetStatus: () => {
            bloomFilterCheck.reset();
            verifyInDatabase.reset();
            uploadFile.reset();
            setUploadProgress(0);
        },
        uploadProgress,
        hashcodeProgress,
        isGeneratingHashCodes,
    };
}