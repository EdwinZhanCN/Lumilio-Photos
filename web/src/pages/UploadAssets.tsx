import PreviewUploadSection from '@/components/UploadAssets/PreviewUploadSection';
import BatchUploadSection from '@/components/UploadAssets/BatchUploadSection.tsx';
import UploadProvider from "@/contexts/UploadContext.tsx";
import ErrorBoundary from "@/ErrorBoundary.tsx";
import ErrorFallBack from "@/pages/ErrorFallBack.tsx";

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
        </UploadProvider>
    );
};

export default UploadPhotos;