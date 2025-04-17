import React, {useRef, useState, useEffect} from 'react';
import FileDropZone from '@/components/UploadAssets/FileDropZone.jsx';
import ProgressIndicator from '@/components/UploadAssets/ProgressIndicator';
import ImagePreviewGrid from '@/components/UploadAssets/ImagePreviewGrid';
import { useUpload } from '@/contexts/UploadContext';
import {useGenerateThumbnail} from "@/hooks/useGenerateThumbnail.jsx";
import ValidateFile from "@/utils/validate-file.js";
import {acceptFileExtensions} from "@/utils/accept-file-extensions.js";

function PreviewUploadSection(){
    // Import general states and methods from the UploadContext
    const {
        files,
        previews,
        maxPreviewFiles,
        setError,
        setPreviews,
        setFiles,
        clearFiles,
        setSuccess,
    } = useUpload();

    // Progress for generating thumbnail, special for this section
    const [genThumbnailProgress, setGenThumbnailProgress] = useState(0);
    const [isGenThumbnails, setIsGenThumbnails] = useState(false);

    // file upload related
    const fileInputRef = useRef(null);
    const [uploadProgress, setUploadProgress] = useState(0);
    const timeoutRef = useRef(null);

    // Cleanup timeouts when component unmounts
    useEffect(() => {
        return () => {
            if (timeoutRef.current) clearTimeout(timeoutRef.current);
        };
    }, []);

    const { generatePreviews } = useGenerateThumbnail({setGenThumbnailProgress, setIsGenThumbnails});

    /**
     * Handle file selection and validation
     * @param {FileList} selectedFiles The files selected by the user
     */
    const handleFiles = (selectedFiles) => {
        const validFiles = Array.from(selectedFiles).filter(file =>
            ValidateFile(file)
        );

        // Apply file limit
        const availableSlots = maxPreviewFiles - files.length;
        if (availableSlots <= 0) {
            setError(`You can only upload at most ${maxPreviewFiles} files`);
            timeoutRef.current = setTimeout(() => setError(''), 3000);
            return;
        }

        const filteredFiles = validFiles.slice(0, availableSlots);
        if (validFiles.length > availableSlots) {
            setError(`The exceeded ${validFiles.length - availableSlots} files have been removed`);
            timeoutRef.current = setTimeout(() => setError(''), 3000);
        }

        setFiles(prev => [...prev, ...filteredFiles]);

        // Initialize previews with null placeholders
        setPreviews(prev => {
            const newPreviews = [...prev];
            // Add null placeholders for new files
            for (let i = 0; i < filteredFiles.length; i++) {
                newPreviews.push(null);
            }
            return newPreviews;
        });

        // Generate previews for the new files
        generatePreviews(filteredFiles);
    };

    // File upload handler
    const handleUpload = async () => {
        if (files.length === 0) {
            setError('Please select photos to upload');
            timeoutRef.current = setTimeout(() => setError(''), 3000);
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

            timeoutRef.current = setTimeout(() => {
                setSuccess('');
                clearFiles();
            }, 2000);
        } catch (err) {
            setError(err.message || 'Upload failed, please try again');
            timeoutRef.current = setTimeout(() => setError(''), 3000);
        }
    };

    // Handler for clearing files
    const handleClear = () => {
        clearFiles();
    };

    return (
        <section id="preview-upload-assets" className="max-w-3xl mx-auto">
            <h1 className="text-3xl font-bold mb-8">Preview Upload Assets</h1>

            <small className="text-sm text-base-content/70">
                This Section is for previewing the photos you want to upload. <br />
                The maximum number of files you can upload is {maxPreviewFiles}. <br />
                You can change the maximum number of files in the system setting.
            </small>

            {/* Drop Zone */}
            <FileDropZone
                fileInputRef={fileInputRef}
                onFilesDropped={handleFiles}
            />

            {/* Click Zone */}
            <input
                type="file"
                ref={fileInputRef}
                className="hidden"
                multiple
                accept={"image/*,video/*," + acceptFileExtensions.join(',')}
                onChange={(e) => {
                    handleFiles(e.target.files);
                }}
            />

            <div className="text-sm text-gray-500 mb-4">
                {files.length} / {maxPreviewFiles} files
                <progress
                    className="ml-2 w-32 h-2 align-middle"
                    value={files.length}
                    max={maxPreviewFiles}
                />
            </div>

            {isGenThumbnails && genThumbnailProgress && (
                <div className="flex gap-4">
                    <ProgressIndicator
                        processed={genThumbnailProgress.numberProcessed}
                        total={genThumbnailProgress.total}
                    />
                    <div className="flex justify-center items-center mb-6">
                        <span className="loading loading-dots loading-md"></span>
                    </div>
                </div>
            )}

            <ImagePreviewGrid previews={previews} />

            {uploadProgress > 0 && (
                <progress
                    className="progress progress-success w-56"
                    value={uploadProgress}
                    max="100"
                >
                </progress>
            )}

            <div className="flex justify-end gap-4">
                <button
                    onClick={handleClear}
                    disabled={files.length === 0 || uploadProgress > 0}
                    className="mb-2 mt-2 btn btn-ghost"
                >
                    Clear
                </button>
                <button
                    onClick={handleUpload}
                    className="mb-2 mt-2 btn btn-primary"
                    disabled={files.length === 0 || uploadProgress > 0}
                >
                    {uploadProgress > 0 ? 'Uploading...' : 'Start Upload'}
                </button>
            </div>
        </section>
    );
};

export default PreviewUploadSection;