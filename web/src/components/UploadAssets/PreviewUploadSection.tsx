import React, {
    useRef,
    useState,
    useEffect,
    ChangeEvent,
} from 'react';
import FileDropZone from '@/components/UploadAssets/FileDropZone.tsx';
import ProgressIndicator from '@/components/UploadAssets/ProgressIndicator';
import ImagePreviewGrid from '@/components/UploadAssets/ImagePreviewGrid';
import { useUploadContext } from '@/contexts/UploadContext';
import { useGenerateThumbnail } from '@/hooks/wasm-hooks/useGenerateThumbnail';
import ValidateFile from '@/utils/validate-file';
import { acceptFileExtensions } from '@/utils/accept-file-extensions';
// import { useUploadProcess } from '@/hooks/api-hooks/useUploadProcess';
import { useMessage } from '@/hooks/util-hooks/useMessage';

// For the thumbnail progress shape (or adapt it if your actual shape differs)
interface ThumbnailProgress {
    numberProcessed: number;
    total: number;
}

function PreviewUploadSection(): React.JSX.Element {
    // Values from your custom context
    const {
        files,
        previews,
        maxPreviewFiles,
        setPreviews,
        setFiles,
        clearFiles,
        workerClientRef,
        wasmReady,
        uploadProgress,
    } = useUploadContext();

    const showMessage = useMessage();

    const [genThumbnailProgress, setGenThumbnailProgress] = useState<ThumbnailProgress | null>(null);
    const [isGenThumbnails, setIsGenThumbnails] = useState<boolean>(false);

    // file upload refs
    const fileInputRef = useRef<HTMLInputElement>(null);
    const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // from a custom hook
    // const { processFiles } = useUploadProcess(workerClientRef, wasmReady);

    useEffect(() => {
        return () => {
            if (timeoutRef.current) {
                clearTimeout(timeoutRef.current);
            }
        };
    }, []);

    const { generatePreviews } = useGenerateThumbnail({
        setGenThumbnailProgress,
        setIsGenThumbnails,
        workerClientRef,
        wasmReady,
        setPreviews,
    });

    /**
     * Handle file selection and validation.
     */
    const handleFiles = (selectedFiles: FileList) => {
        const validFiles = Array.from(selectedFiles).filter((file) => ValidateFile(file));
        const availableSlots = maxPreviewFiles - files.length;

        if (availableSlots <= 0) {
            showMessage('hint', `You can only upload at most ${maxPreviewFiles} files`);
            return;
        }

        const filteredFiles = validFiles.slice(0, availableSlots);
        if (validFiles.length > availableSlots) {
            showMessage('error', `The exceeded ${validFiles.length - availableSlots} files have been removed`);
        }

        setFiles((prev) => [...prev, ...filteredFiles]);

        // Initialize placeholders
        setPreviews((prev) => [
            ...prev,
            ...filteredFiles.map(() => null),
        ]);


        generatePreviews(filteredFiles).catch((error) => {
            console.error('Thumbnail generation failed:', error);
            showMessage('error', 'Failed to generate previews');
            clearFiles(fileInputRef);
        });
    };

    const handleClear = () => {
        clearFiles(fileInputRef);
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
                onFilesDropped={handleFiles} children={undefined}
            />

            {/* Click Zone */}
            <input
                type="file"
                ref={fileInputRef}
                className="hidden"
                multiple
                accept={`image/*,video/*,${acceptFileExtensions.join(',')}`}
                onChange={(e: ChangeEvent<HTMLInputElement>) => {
                    if (e.target.files) {
                        handleFiles(e.target.files);
                        e.target.value = ''; // 添加这行重置input值
                    }
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
                        <span className="loading loading-dots loading-md" />
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
                    Upload Progress
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
                    onClick={() => {}}
                    className="mb-2 mt-2 btn btn-primary"
                    disabled={files.length === 0 || uploadProgress > 0}
                >
                    {uploadProgress > 0 ? 'Uploading...' : 'Start Upload'}
                </button>
            </div>
        </section>
    );
}

export default PreviewUploadSection;