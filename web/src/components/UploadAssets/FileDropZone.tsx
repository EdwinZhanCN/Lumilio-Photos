import React from 'react';
import { useUploadContext } from '@/contexts/UploadContext.js';


type FileDropZoneProps = {
    fileInputRef: React.RefObject<HTMLInputElement|null>;
    children?: React.ReactNode;
    onFilesDropped: (files: FileList) => void;
};

const FileDropZone = ({fileInputRef, children, onFilesDropped}:FileDropZoneProps) => {
    const {
        isDragging,
        handleDragOver,
        handleDragLeave,
        handleDrop
    } = useUploadContext();

    // Create a wrapper for handleDrop that calls the specific file handler
    const onDrop = (e:React.DragEvent<Element>) => {
        handleDrop(e, onFilesDropped);
    };

    return (
        <div
            className={`mt-5 border-2 border-dashed rounded-xl p-8 mb-6 text-center transition-colors
                ${isDragging ? 'border-blue-500 bg-blue-50' : 'border-gray-300 hover:border-blue-400'}`}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={onDrop}
            onClick={() => fileInputRef.current?.click()}
        >
            <div className="space-y-4">
                <svg
                    className="mx-auto h-12 w-12 text-gray-400"
                    stroke="currentColor"
                    fill="none"
                    viewBox="0 0 48 48"
                >
                    <path
                        d="M28 8H12a4 4 0 00-4 4v20m32-12v8m0 0v8a4 4 0 01-4 4H12a4 4 0 01-4-4v-4m32-4l-3.172-3.172a4 4 0 00-5.656 0L28 28M8 32l9.172-9.172a4 4 0 015.656 0L28 28m0 0l4 4m4-24h8m-4-4v8m-12 4h.02"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    />
                </svg>
                <div className="text-base-content/50">
                    {children || (
                        <>
                            <p className="font-medium">Drag or Click Here to Upload</p>
                            <p className="text-sm">Supports JPEG, PNG, RAW</p>
                        </>
                    )}
                </div>
            </div>
        </div>
    );
};

export default FileDropZone;