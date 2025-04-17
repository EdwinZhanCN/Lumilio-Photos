import React from 'react';
import PreviewUploadSection from '@/components/UploadAssets/PreviewUploadSection';
import BatchUploadSection from '@/components/UploadAssets/BatchUploadSection.jsx';
import UploadProvider from "@/contexts/UploadContext.jsx";
import UploadNotifications from "@/components/UploadAssets/UploadNotifications.jsx";
import ErrorBoundary from "@/ErrorBoundary.jsx";
import ErrorFallBack from "@/pages/ErrorFallBack.jsx";

const UploadPhotos = () => {
    return (
        <UploadProvider>
            <ErrorBoundary
                fallback={({resetErrorBoundary}) => (
                    <ErrorFallBack
                        code = {"500"}
                        title={"Batch Upload Section Error!"}
                        message={"The preview upload section failed to load or encountered an error during operation."}
                        reset={resetErrorBoundary}
                    />
                )}
            >
                <PreviewUploadSection/>
            </ErrorBoundary>
            <ErrorBoundary
                fallback={({resetErrorBoundary}) => (
                    <ErrorFallBack
                        code = {"500"}
                        title={"Batch Upload Section Error!"}
                        message={"The batch upload section failed to load or encountered an error during operation."}
                        reset={resetErrorBoundary}
                    />
                )}
            >
                <BatchUploadSection/>
            </ErrorBoundary>
            <UploadNotifications/>
        </UploadProvider>
    );
};

export default UploadPhotos;