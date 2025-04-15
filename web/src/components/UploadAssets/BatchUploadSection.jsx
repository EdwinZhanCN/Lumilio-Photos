import React from 'react';
import ProgressIndicator from '@/components/UploadAssets/ProgressIndicator';

const BatchUploadSection = ({fileInputRef, isGeneratingHashCodes, hashcodeProgress}) => {
    return (
        <section id="batch-upload-photos" className="max-w-3xl mx-auto">
            <h1 className="text-3xl font-bold">Batch Upload Photos</h1>
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

            {/*Button Handle file Input*/}
            <button onClick={event => fileInputRef.current.click()} className="mb-2 mt-2 btn btn-primary">Batch Upload</button>
        </section>
    );
};

export default BatchUploadSection;