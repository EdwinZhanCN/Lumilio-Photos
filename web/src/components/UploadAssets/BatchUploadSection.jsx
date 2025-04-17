import React, {useRef, useState, useEffect} from 'react';
import ProgressIndicator from '@/components/UploadAssets/ProgressIndicator';
import {useGenerateHashcode} from "@/hooks/useGenerateHashcode.jsx";
import {useUpload} from "@/contexts/UploadContext.jsx";
import { formatBytes } from '@/utils/formatters';
import {acceptFileExtensions} from "@/utils/accept-file-extensions.js";

function BatchUploadSection(){
    const {
        setError,
        setSuccess,
        setHint,
    } = useUpload();

    const [hashcodeProgress, setHashcodeProgress] = useState(null);
    const [isGeneratingHashCodes, setIsGeneratingHashCodes] = useState(false);
    const [uploadProgress, setUploadProgress] = useState(null);
    const [isUploading, setIsUploading] = useState(false);
    const fileInputRef = useRef(null);
    const successTimeoutRef = useRef(null);
    const errorTimeoutRef = useRef(null);


    const handlePerformanceMetrics = (metrics) => {
        const formattedSpeed = formatBytes(metrics.bytesPerSecond) + '/s';
        const timeInSeconds = (metrics.processingTime / 1000).toFixed(2);
        const formattedSize = formatBytes(metrics.totalSize);
        // Timeout to clear the hint after 5 seconds
        setHint(`Processed ${metrics.numberProcessed} files (${formattedSize}) in ${timeInSeconds}s at ${formattedSpeed}`);
        successTimeoutRef.current = setTimeout(() => setHint(''), 5000);
    };

    const { generateHashCodes } = useGenerateHashcode({
        setIsGeneratingHashCodes,
        setHashcodeProgress,
        onPerformanceMetrics: handlePerformanceMetrics,
    });

    /**
     * Upload files with their hash results in chunks
     * @param {FileList} files
     * @param {Array<{index: number, hash: string}>} hashResults
     */
    const batchUploadAssets = async (files, hashResults) => {
        if (!hashResults) {
            setError("Cannot upload files without hash codes");
            return;
        }

        try {
            // Map files to their hash results for chunk upload
            const fileHashMap = Array.from(files).map((file, index) => ({
                file,
                hash: hashResults[index]?.hash,
                size: file.size
            }));

            setIsUploading(true);
            setUploadProgress({ processed: 0, total: fileHashMap.length });

            // Implementation of chunk upload logic
            for (let i = 0; i < fileHashMap.length; i++) {
                const { file, hash, size } = fileHashMap[i];

                // Here you would implement your actual upload logic
                // For example:
                // await uploadFileInChunks(file, hash);

                // Update progress
                setUploadProgress({ processed: i + 1, total: fileHashMap.length });
            }

            setSuccess(`Successfully uploaded ${fileHashMap.length} files`);
            successTimeoutRef.current = setTimeout(() => setSuccess(''), 5000);
        } catch (err) {
            setError(`Upload failed: ${err.message}`);
            errorTimeoutRef.current = setTimeout(() => setError(''), 3000);
        } finally {
            setIsUploading(false);
        }
    };

    const handleFileChange = async (e) => {
        const files = e.target.files;
        if (!files || files.length === 0) return;

        try {
            const hashResults = await generateHashCodes(files);

            if (!hashResults) {
                return;
            }

            if (hashResults instanceof Error) {
                setError(hashResults.message);
                errorTimeoutRef.current = setTimeout(() => setError(''), 3000);
                return;
            }

            await batchUploadAssets(files, hashResults);
        } catch (err) {
            setError(`Error processing files: ${err.message}`);
            errorTimeoutRef.current = setTimeout(() => setError(''), 3000);
        }
    };

    return (
        <section id="batch-upload-assets" className="max-w-3xl mx-auto">
            <h1 className="text-3xl font-bold">Batch Upload Assets</h1>
            <small className="text-sm text-base-content/70">
                This Section is for large files like RAW, Video to upload. <br />
            </small>

            {isGeneratingHashCodes && hashcodeProgress && (
                <ProgressIndicator
                    processed={hashcodeProgress.numberProcessed}
                    total={hashcodeProgress.total}
                    label="Generating hashes"
                />
            )}

            {isUploading && uploadProgress && (
                <ProgressIndicator
                    processed={uploadProgress.processed}
                    total={uploadProgress.total}
                    label="Uploading files"
                />
            )}

            <button
                onClick={() => fileInputRef.current.click()}
                className="mb-2 mt-2 btn btn-primary"
                disabled={isGeneratingHashCodes || isUploading}
            >
                {isGeneratingHashCodes ? 'Generating Hashes...' :
                    isUploading ? 'Uploading...' : 'Batch Upload'}
            </button>

            <input
                type="file"
                ref={fileInputRef}
                className="hidden"
                multiple
                accept={"image/*,video/*," + acceptFileExtensions.join(',')}
                onChange={handleFileChange}
            />
        </section>
    );
};

export default BatchUploadSection;