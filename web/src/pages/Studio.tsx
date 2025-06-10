import React, { useState, useRef, useEffect, useCallback } from 'react';
import { WorkerClient } from '@/workers/workerClient.ts';
import { useExtractExifdata } from '@/hooks/util-hooks/useExtractExifdata.tsx';
import { ArrowUpTrayIcon, DocumentTextIcon, ExclamationCircleIcon } from '@heroicons/react/24/outline';

export function Studio() {
    const [selectedFile, setSelectedFile] = useState<File | null>(null);
    const [imageUrl, setImageUrl] = useState<string | null>(null);

    // State for the hook to populate (Record<number, any>)
    const [rawExifDataFromHook, setRawExifDataFromHook] = useState<Record<number, any> | null>(null);
    // State for the single image's EXIF data to display (Record<string, any>)
    const [exifToDisplay, setExifToDisplay] = useState<Record<string, any> | null>(null);

    const [isExtracting, setIsExtracting] = useState(false);
    const [progress, setProgress] = useState<{
        numberProcessed: number;
        total: number;
        error?: string;
        failedAt?: number | null;
    } | null>(null);

    const workerClientRef = useRef<WorkerClient | null>(null);
    const fileInputRef = useRef<HTMLInputElement>(null);

    // Initialize WorkerClient
    useEffect(() => {
        workerClientRef.current = new WorkerClient();
        // console.log("WorkerClient initialized");
        return () => {
            // console.log("Terminating EXIF worker");
            workerClientRef.current?.terminateExtractExifWorker();
        };
    }, []);

    // Instantiate the hook
    const { extractExifData } = useExtractExifdata({
        workerClientRef,
        setExtractExifProgress: setProgress,
        setIsExtractingExif: setIsExtracting,
        setExifData: setRawExifDataFromHook, // Hook updates this state
    });

    // Process raw EXIF data from the hook when it changes
    useEffect(() => {
        if (rawExifDataFromHook && rawExifDataFromHook[0]) {
            // Assuming the first item (index 0) is our single image's data
            setExifToDisplay(rawExifDataFromHook[0]);
        } else {
            setExifToDisplay(null);
        }
    }, [rawExifDataFromHook]);

    const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        if (file) {
            setSelectedFile(file);
            const reader = new FileReader();
            reader.onloadend = () => {
                setImageUrl(reader.result as string);
            };
            reader.readAsDataURL(file);
            setExifToDisplay(null); // Reset previous EXIF data
            setProgress(null); // Reset progress
            setRawExifDataFromHook(null); // Reset raw data
        }
    };

    const handleExtractExif = useCallback(async () => {
        if (selectedFile && workerClientRef.current) {
            // console.log("Starting EXIF extraction for:", selectedFile.name);
            setExifToDisplay(null); // Clear previous results
            await extractExifData([selectedFile]);
        } else {
            // console.log("No file selected or worker not ready");
            // Optionally, show a message to the user via useMessage if integrated
        }
    }, [selectedFile, extractExifData]);

    const triggerFileInput = () => {
        fileInputRef.current?.click();
    };

    const renderExifData = () => {
        if (!exifToDisplay) return null;

        const entries = Object.entries(exifToDisplay);
        if (entries.length === 0) {
            return <p className="text-center text-gray-500">No EXIF data found for this image.</p>;
        }

        // Filter out specific large binary data like ThumbnailImage for better display
        const filteredEntries = entries.filter(([key]) => !key.toLowerCase().includes('thumbnailimage'));

        return (
            <div className="overflow-x-auto max-h-96">
                <table className="table table-zebra table-sm w-full">
                    <thead>
                        <tr>
                            <th className="sticky top-0 bg-base-200 z-10">Tag</th>
                            <th className="sticky top-0 bg-base-200 z-10">Value</th>
                        </tr>
                    </thead>
                    <tbody>
                        {filteredEntries.map(([key, value]) => (
                            <tr key={key}>
                                <td className="font-semibold break-all">{key}</td>
                                <td className="break-all">
                                    {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        );
    };

    return (
        <div className="container mx-auto p-4 md:p-8 min-h-screen bg-base-100">
            <header className="mb-8 text-center">
                <h1 className="text-4xl font-bold text-primary">Image EXIF Studio</h1>
                <p className="text-lg text-base-content/70">Select an image to view its EXIF metadata.</p>
            </header>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                {/* Image Selection and Preview Card */}
                <div className="card bg-base-200 shadow-xl">
                    <div className="card-body">
                        <h2 className="card-title text-2xl mb-4">
                            <ArrowUpTrayIcon className="w-6 h-6 mr-2"/>
                            Select Image
                        </h2>
                        <input
                            type="file"
                            ref={fileInputRef}
                            onChange={handleFileChange}
                            accept="image/jpeg, image/png, image/tiff, image.heic, image.heif, image.webp"
                            className="hidden"
                        />
                        <button
                            onClick={triggerFileInput}
                            className="btn btn-primary w-full mb-4"
                        >
                            Choose Image
                        </button>

                        {imageUrl && selectedFile && (
                            <div className="mt-4">
                                <h3 className="text-xl font-semibold mb-2">Preview:</h3>
                                <img src={imageUrl} alt={selectedFile.name} className="rounded-lg shadow-md max-h-96 w-full object-contain"/>
                                <div className="mt-4 flex flex-col space-y-2">
                                    <p className="text-sm"><span className="font-semibold">File:</span> {selectedFile.name}</p>
                                    <p className="text-sm"><span className="font-semibold">Size:</span> {(selectedFile.size / 1024).toFixed(2)} KB</p>
                                    <button
                                        onClick={handleExtractExif}
                                        className="btn btn-secondary w-full mt-2"
                                        disabled={isExtracting}
                                    >
                                        {isExtracting ? 'Extracting...' : 'Extract EXIF Data'}
                                    </button>
                                </div>
                            </div>
                        )}

                        {isExtracting && progress && (
                            <div className="mt-4">
                                <p className="text-sm text-center mb-1">Processing: {progress.numberProcessed} / {progress.total}</p>
                                <progress
                                    className="progress progress-primary w-full"
                                    value={progress.numberProcessed}
                                    max={progress.total}
                                ></progress>
                            </div>
                        )}

                        {progress?.error && (
                            <div role="alert" className="alert alert-error mt-4">
                                <ExclamationCircleIcon className="w-6 h-6"/>
                                <div>
                                    <h3 className="font-bold">Error!</h3>
                                    <div className="text-xs">{progress.error}</div>
                                 </div>
                            </div>
                        )}
                    </div>
                </div>

                {/* EXIF Data Display Card */}
                {(exifToDisplay || isExtracting || progress?.error) && (
                    <div className="card bg-base-200 shadow-xl">
                        <div className="card-body">
                            <h2 className="card-title text-2xl mb-4">
                                <DocumentTextIcon className="w-6 h-6 mr-2"/>
                                EXIF Metadata
                            </h2>
                            {isExtracting && !exifToDisplay && !progress?.error && (
                                <div className="text-center py-8">
                                    <span className="loading loading-lg loading-spinner text-primary"></span>
                                    <p className="mt-2">Extracting data...</p>
                                </div>
                            )}
                            {!isExtracting && renderExifData()}
                        </div>
                    </div>
                )}
            </div>
             <footer className="text-center mt-12 py-4 border-t border-base-300">
                <p className="text-sm text-base-content/60">Image EXIF Studio &copy; 2025</p>
            </footer>
        </div>
    );
}

