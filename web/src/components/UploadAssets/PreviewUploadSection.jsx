import React from 'react';
import FileUploadZone from '@/components/UploadAssets/FileUploadZone';
import ProgressIndicator from '@/components/UploadAssets/ProgressIndicator';
import ImagePreviewGrid from '@/components/UploadAssets/ImagePreviewGrid';

const PreviewUploadSection = ({
                                  wasmReady,
                                  maxFiles,
                                  files,
                                  isDragging,
                                  handleDragOver,
                                  handleDragLeave,
                                  handleDrop,
                                  fileInputRef,
                                  isGeneratingThumbnails,
                                  thumbnailProgress,
                                  previews,
                                  uploadProgress,
                                  handleUpload,
                                  clearFiles
                              }) => {
    return (
        <section id="preview-upload-photos" className="max-w-3xl mx-auto">
            <h1 className="text-3xl font-bold mb-8">Preview Upload Photos</h1>
            {!wasmReady && (
                <div className="text-xs text-amber-500 mt-1">
                    WebAssembly module is loading...
                </div>
            )}
            <small className="text-sm text-base-content/70">
                This Section is for previewing the photos you want to upload. <br />
                The maximum number of files you can upload is {maxFiles}. <br />
                You can change the maximum number of files in the system setting.
            </small>

            <FileUploadZone
                isDragging={isDragging}
                handleDragOver={handleDragOver}
                handleDragLeave={handleDragLeave}
                handleDrop={handleDrop}
                fileInputRef={fileInputRef}
            />

            <div className="text-sm text-gray-500 mb-4">
                {files.length} / {maxFiles} files
                <progress
                    className="ml-2 w-32 h-2 align-middle"
                    value={files.length}
                    max={maxFiles}
                />
            </div>

            {isGeneratingThumbnails && thumbnailProgress && (
                <div className="flex gap-4">
                    <ProgressIndicator
                        processed={thumbnailProgress.numberProcessed}
                        total={thumbnailProgress.total}
                    />
                    <div className="flex justify-center items-center mb-6">
                        <span className="loading loading-dots loading-md"></span>
                    </div>
                </div>
            )}

            <ImagePreviewGrid previews={previews} />

            {uploadProgress > 0 && (
                <div className="mb-4">
                    <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                        <div
                            className="h-full bg-blue-500 transition-all duration-300"
                            style={{ width: `${uploadProgress}%` }}
                        />
                    </div>
                    <p className="text-sm text-gray-600 mt-1">
                        Uploading... {Math.min(uploadProgress, 99)}%
                    </p>
                </div>
            )}

            <div className="flex justify-end gap-4">
                <button
                    onClick={clearFiles}
                    className="px-4 py-2 text-base-content/50 hover:text-base-content disabled:opacity-50"
                    disabled={files.length === 0 || uploadProgress > 0}
                >
                    Clear
                </button>
                <button
                    onClick={handleUpload}
                    className="px-6 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 disabled:opacity-50 transition-colors
          hover:cursor-pointer disabled:cursor-not-allowed"
                    disabled={files.length === 0 || uploadProgress > 0}
                >
                    {uploadProgress > 0 ? 'Uploading...' : 'Start Upload'}
                </button>
            </div>
        </section>
    );
};

export default PreviewUploadSection;