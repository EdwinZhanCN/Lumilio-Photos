import React, {useRef} from 'react';
import ProgressIndicator from '@/components/UploadAssets/ProgressIndicator';
import {acceptFileExtensions} from "@/utils/accept-file-extensions.js";
import {useUploadContext} from "@/contexts/UploadContext.jsx";

function BatchUploadSection(){


    const fileInputRef = useRef(null);
    // TODO Handel file change

    const {
        BatchUpload,
        /**
         * @type {boolean}
         */
        isGeneratingHashCodes,
        /**
         * @type {boolean}
         */
        isUploading,
        uploadProgress,
        hashcodeProgress,
    } = useUploadContext();

    const handleFileChange = (uploadFiles) => {
        if (uploadFiles.length > 0) {
            BatchUpload(uploadFiles);
        }
    }


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
                onChange={event => {
                    handleFileChange(event.target.files);
                }}
            />
        </section>
    );
}

export default BatchUploadSection;