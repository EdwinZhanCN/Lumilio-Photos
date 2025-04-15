// Updated UploadPhotos.jsx
import React, { useState, useRef, useCallback, useEffect } from 'react';
import { WasmWorkerClient } from "@/workers/workerClient.js";
import { useGenerateThumbnail } from "@/hooks/useGenerateThumbnail.jsx";
import { useGenerateHashcode } from "@/hooks/useGenerateHashcode.jsx";
import { formatBytes } from '../utils/formatters';
import PreviewUploadSection from '@/components/UploadAssets/PreviewUploadSection';
import BatchUploadSection from '@/components/UploadAssets/BatchUploadSection.jsx';

const UploadPhotos = () => {
    const rawFileExtensions = ['.raw', '.cr2', '.nef', '.orf', '.sr2',
        '.arw', '.rw2', '.dng', '.k25', '.kdc', '.mrw', '.pef', '.raf', '.3fr', '.fff'];

    const [files, setFiles] = useState([]);
    const [previews, setPreviews] = useState([]);

    // User feedback
    const [isDragging, setIsDragging] = useState(false);
    const [uploadProgress, setUploadProgress] = useState(0);
    const fileInputRef = useRef(null);
    const BatchFileInputRef = useRef(null);

    // Error/Success Status display
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const [maxFiles] = useState(30);

    // Wasm Initialization
    const [wasmReady, setWasmReady] = useState(false);

    // States for tracking progress
    const [thumbnailProgress, setThumbnailProgress] = useState(null);
    const [isGeneratingThumbnails, setIsGeneratingThumbnails] = useState(false);
    const [hashcodeProgress, setHashcodeProgress] = useState(null);
    const [isGeneratingHashCodes, setIsGeneratingHashCodes] = useState(false);

    // Use web worker
    const workerClientRef = useRef(null);

    // Initialize hooks
    const { generatePreviews } = useGenerateThumbnail({
        workerClientRef,
        wasmReady,
        setError,
        setPreviews,
        setIsGeneratingThumbnails,
        setThumbnailProgress,
    });

    /**
     * Handle performance metrics after hash generation
     * @param {
     * {
     * startTime:number,
     * fileCount:number,
     * totalSize:number,
     * processingTime:number,
     * filesPerSecond:number,
     * bytesPerSecond:number
     * }
     * }metrics
     */
    const handlePerformanceMetrics = (metrics) => {
        // Format metrics for user-friendly display
        const formattedSpeed = formatBytes(metrics.bytesPerSecond) + '/s';
        const timeInSeconds = (metrics.processingTime / 1000).toFixed(2);
        const formattedSize = formatBytes(metrics.totalSize);

        // Show toast with performance information
        setSuccess(`Hash generation complete! Processed ${metrics.fileCount} files (${formattedSize}) in ${timeInSeconds}s (${formattedSpeed})`);

        setTimeout(() => setSuccess(''), 5000);
    };

    /**
     * Generate hash codes for the selected files
     * @returns {Array<{index: number, hash: string}>|Error} The hash result with the index or an Error
     */
    const { generateHashCodes } = useGenerateHashcode({
        workerClientRef,
        wasmReady,
        setError,
        setIsGeneratingHashCodes,
        setHashcodeProgress,
        onPerformanceMetrics: handlePerformanceMetrics,
    });

    // Clean up generated URLs
    const revokePreviews = useCallback((urls) => {
        urls.forEach(url => {
            if (url) URL.revokeObjectURL(url);
        });
    }, []);

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

    // File type validation
    const isValidFileType = (file) => {
        const supportedImageTypes = [
            'image/',
            'image/x-canon-cr2',
            'image/x-nikon-nef',
            'image/x-sony-arw',
            'image/x-adobe-dng',
            'image/x-fuji-raf',
            'image/x-panasonic-rw2'
        ];

        const supportedVideoTypes = [
            'video/mp4',
            'video/quicktime',
            'video/x-msvideo',
            'video/x-matroska',
            'video/avi',
            'video/mpeg'
        ];

        return supportedImageTypes.some(type =>
            file.type.startsWith(type) ||
            supportedVideoTypes.includes(file.type)
        );
    };

    /**
     * Handle file selection and validation
     * @param {FileList}selectedFiles The files selected by the user
     */
    const handleFiles = (selectedFiles) => {
        const validFiles = Array.from(selectedFiles).filter(file =>
            isValidFileType(file)
        );

        // Apply file limit
        const availableSlots = maxFiles - files.length;
        if (availableSlots <= 0) {
            setError(`You can only upload at most ${maxFiles} files`);
            setTimeout(() => setError(''), 3000);
            return;
        }

        const filteredFiles = validFiles.slice(0, availableSlots);
        if (validFiles.length > availableSlots) {
            setError(`The exceeded ${availableSlots} files have been removed`);
            setTimeout(() => setError(''), 3000);
        }

        setFiles(prev => [...prev, ...filteredFiles]);
        setPreviews(prev => {
            const newPreviews = [...prev];
            // Add null placeholders for new files
            for (let i = 0; i < filteredFiles.length; i++) {
                newPreviews.push(null);
            }
            return newPreviews;
        });
        generatePreviews(filteredFiles);
    };

    // Drag and drop handlers
    const handleDragOver = (e) => {
        e.preventDefault();
        setIsDragging(true);
    };

    const handleDragLeave = (e) => {
        e.preventDefault();
        setIsDragging(false);
    };

    const handleDrop = (e) => {
        e.preventDefault();
        setIsDragging(false);
        const droppedFiles = e.dataTransfer.files;
        handleFiles(droppedFiles);
    };

    // File upload handler
    const handleUpload = async () => {
        if (files.length === 0) {
            setError('Please select photos to upload');
            setTimeout(() => setError(''), 3000);
            return;
        }

        try {
            setUploadProgress(0);
            const formData = new FormData();
            files.forEach(file => {
                formData.append('files', file);
            });

            const response = await fetch(`/api/photos/batch`, {
                method: 'POST',
                body: formData,
            });

            if (!response.ok) {
                throw new Error('Batch upload failed');
            }

            const result = await response.json();
            setUploadProgress(100);
            setSuccess(`Successfully uploaded ${result.data.successful} of ${result.data.total} photos!`);

            setTimeout(() => {
                setSuccess('');
                clearFiles();
            }, 2000);
        } catch (err) {
            setError(err.message || 'Upload failed, please try again');
            setTimeout(() => setError(''), 3000);
        }
    };

    // Clear files and reset state
    const clearFiles = () => {
        revokePreviews(previews);
        setFiles([]);
        setPreviews([]);
        setUploadProgress(0);

        // Reset file input values
        if (fileInputRef.current) {
            fileInputRef.current.value = "";
        }
        if (BatchFileInputRef.current) {
            BatchFileInputRef.current.value = "";
        }
    };

    /**
     * This function first handle the returned HashResult that store in the useState
     * Then it maps the files to their hash results for chunk upload.
     * Finally, Call the chunk upload function
     * @param {FileList}files
     * @param {{index:number,hash:string}[] || {index:number,hash:string}[]}hashResults - the hash result with the index
     */
    const BatchUploadAssets = (files, hashResults) =>{
        if (hashResults) {
            // Map files to their hash results for chunk upload
            const fileHashMap = Array.from(files).map((file, index) => (
                {
                    file,
                    hash: hashResults[index]?.hash,
                    size: file.size
                }
            ));

            // You can now implement chunk upload logic using the mapped data
            console.log('Files with hash codes ready for chunk upload:', fileHashMap);
            // TODO: Implement chunk upload function
        } else {
            setError("Cannot upload files without hash codes");
        }
    }

    return (
        <div className="min-h-screen px-2">
            <PreviewUploadSection
                wasmReady={wasmReady}
                maxFiles={maxFiles}
                files={files}
                isDragging={isDragging}
                handleDragOver={handleDragOver}
                handleDragLeave={handleDragLeave}
                handleDrop={handleDrop}
                fileInputRef={fileInputRef}
                isGeneratingThumbnails={isGeneratingThumbnails}
                thumbnailProgress={thumbnailProgress}
                previews={previews}
                uploadProgress={uploadProgress}
                handleUpload={handleUpload}
                clearFiles={clearFiles}
            />

            <BatchUploadSection
                isDragging={isDragging}
                handleDragOver={handleDragOver}
                handleDragLeave={handleDragLeave}
                handleDrop={handleDrop}
                // This ref is different from the one used in PreviewUploadSection
                fileInputRef={BatchFileInputRef}
                isGeneratingHashCodes={isGeneratingHashCodes}
                hashcodeProgress={hashcodeProgress}
            />

            {/* Hidden file input that both sections use */}
            <input
                type="file"
                ref={fileInputRef}
                className="hidden"
                multiple
                accept={"image/*,video/*,.cr2, .nef, .arw, .raf, .rw2, .dng,.mov, .mp4, .avi, .mkv," + rawFileExtensions.join(',')}
                onChange={(e) => {
                    handleFiles(e.target.files);
                }}
            />

            <input
                type="file"
                ref={BatchFileInputRef}
                className="hidden"
                multiple
                accept={"image/*,video/*,.cr2, .nef, .arw, .raf, .rw2, .dng,.mov, .mp4, .avi, .mkv"+rawFileExtensions}
                onChange={(e) => {
                    // This generateHashCodes function will return a hash result with the index [{index:number,hash:string},...]
                    // Or Error
                    generateHashCodes(e.target.files).then(
                        r => {
                            // r:[{index:number,hash:string},...]
                            if (r){
                                if (r instanceof Error){
                                    setError(r.message);
                                    setTimeout(() => setError(''), 3000);
                                    throw new Error("An error occurred during hash code generation.");
                                }else {
                                    BatchUploadAssets(e.target.files, r)
                                }
                            }
                        }
                    );
                }}
            />



            {/* Toast notifications */}
            {error && (
                <div className="toast toast-top toast-right duration-500">
                    <div className="alert alert-error">
                        {error}
                    </div>
                </div>
            )}

            {success && (
                <div className="toast toast-top toast-right duration-500">
                    <div className="alert alert-success">
                        {success}
                    </div>
                </div>
            )}
        </div>
    );
};

export default UploadPhotos;