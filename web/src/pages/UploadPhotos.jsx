import {useState, useRef, useCallback, useEffect} from 'react';
import { WasmWorkerClient } from "@/workers/workerClient.js";
// const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';
import React from 'react';

const UploadPhotos = () => {
    const [wasmError, setWasmError] = useState(false);
    const rawFileExtensions = ['.raw', '.cr2', '.nef', '.orf', '.sr2',
        '.arw', '.rw2', '.dng', '.k25', '.kdc', '.mrw', '.pef', '.raf', '.3fr', '.fff'];
    const [files, setFiles] = useState([]);
    const [previews, setPreviews] = useState([]);
    const [isDragging, setIsDragging] = useState(false);
    const [uploadProgress, setUploadProgress] = useState(0);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const fileInputRef = useRef(null);
    const [maxFiles] = useState(30);

    // New state for tracking thumbnail generation progress
    const [thumbnailProgress, setThumbnailProgress] = useState(null);
    const [isGeneratingThumbnails, setIsGeneratingThumbnails] = useState(false);
    const [wasmReady, setWasmReady] = useState(false);

    const workerClientRef = useRef(null);


    // 清理所有生成的URL
    const revokePreviews = useCallback((urls) => {
        urls.forEach(url => URL.revokeObjectURL(url));
    }, []);

    //#region Legacy Compress Image
    const legacyCompressImage = useCallback(async (file) => {
        return new Promise((resolve, reject) => {
            const img = new Image();
            const reader = new FileReader();

            reader.onload = (e) => {
                img.onload = async () => {
                    try {
                        const canvas = new OffscreenCanvas(img.width, img.height);
                        const ctx = canvas.getContext('2d');
                        ctx.drawImage(img, 0, 0);

                        // 调整尺寸
                        const MAX_SIZE = 300;
                        let width = img.width;
                        let height = img.height;

                        if (width > height && width > MAX_SIZE) {
                            height = Math.round(height * MAX_SIZE / width);
                            width = MAX_SIZE;
                        } else if (height > MAX_SIZE) {
                            width = Math.round(width * MAX_SIZE / height);
                            height = MAX_SIZE;
                        }

                        // 高质量缩放
                        const resizedCanvas = new OffscreenCanvas(width, height);
                        const resizedCtx = resizedCanvas.getContext('2d');
                        resizedCtx.drawImage(canvas, 0, 0, width, height);

                        // 转换为Blob
                        const blob = await resizedCanvas.convertToBlob({
                            type: 'image/jpeg',
                            quality: 0.7
                        });

                        resolve(URL.createObjectURL(blob));
                    } catch (error) {
                        reject(error);
                    }
                };

                img.onerror = reject;
                img.src = e.target.result;
            };

            reader.onerror = reject;
            reader.readAsDataURL(file);
        });
    },[]);
    //#endregion

    // Initialize worker client
    useEffect(() => {
        if (!workerClientRef.current) {
            // relative path to the workerClient.js
            const workerUrl = new URL('../workers/thumbnail.worker.js', import.meta.url);
            workerClientRef.current = new WasmWorkerClient(workerUrl);
        }

        // Initialize WASM
        const initWasm = async () => {
            try {
                await workerClientRef.current.initWASM();
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
                workerClientRef.current.terminate();
            }
        };
    }, []);

    const BATCH_SIZE = 10;

    const generatePreviews = useCallback(async (files) => {
        if (!workerClientRef.current || !wasmReady) {
            setError('WebAssembly module is not ready yet');
            return;
        }

        try {
            setIsGeneratingThumbnails(true);

            const startIndex = previews.length;
            setPreviews(prev => [...prev, ...Array(files.length).fill(null)]);
            const removeProgressListener = workerClientRef.current.addProgressListener(({ processed }) => {
                setThumbnailProgress(prev => ({
                    ...prev,
                    numberProcessed: processed,
                    total: files.length
                }));
            });
    

            // Process files in smaller batches for better performance
            const fileArray = Array.from(files);

            for (let i = 0; i < fileArray.length; i += BATCH_SIZE) {
                const batch = fileArray.slice(i, i + BATCH_SIZE);
                const batchIndex = i / BATCH_SIZE;

                // Call worker client directly
                const result = await workerClientRef.current.generateThumbnail({
                    files: batch,
                    batchIndex: batchIndex,
                    startIndex: startIndex + i
                });
                if (result.status === 'complete' && result.results) {
                    setPreviews(prev => {
                        const newPreviews = [...prev];
                        result.results.forEach(({ index, url }) => {
                            const actualIndex = startIndex + index;
                            if (url && actualIndex < newPreviews.length) {
                                newPreviews[actualIndex] = url;
                            } else {
                                console.warn('Invalid preview index:', actualIndex);
                            }
                        });
                        return newPreviews;
                    });
                }
            }
        } catch (error) {
            console.error('Error generating thumbnails:', error);
            setError(`Thumbnail generation failed: ${error?.message || 'Unknown error'}`);
            setThumbnailProgress(prev => ({
              ...prev,
              error: error?.message,
              failedAt: Date.now()
            }));
        } finally {
            setIsGeneratingThumbnails(false);
            removeProgressListener(); 
            // Reset progress only after all batches complete
            console.log('All batches processed - clearing progress');
            setThumbnailProgress(null);
        }
    }, [previews, wasmReady]);


    // 处理文件类型
    const isValidFileType = (file) => {
        const supportedImageTypes = [
            'image/',
            'image/x-canon-cr2',    // Canon RAW
            'image/x-nikon-nef',    // Nikon RAW
            'image/x-sony-arw',     // Sony RAW
            'image/x-adobe-dng',    // Adobe DNG
            'image/x-fuji-raf',     // Fujifilm RAF
            'image/x-panasonic-rw2' // Panasonic RW2
        ];

        const supportedVideoTypes = [
            'video/mp4',
            'video/quicktime',      // MOV
            'video/x-msvideo',      // AVI
            'video/x-matroska',     // MKV
            'video/avi',
            'video/mpeg'
        ];

        // 检查是否是支持的图片/RAW或视频类型
        return supportedImageTypes.some(type =>
            file.type.startsWith(type) ||
            supportedVideoTypes.includes(file.type)
        );
    };

    /**
     * Handle file selection
     * @param selectedFiles {FileList} - The selected files from the input
     */
    const handleFiles = (selectedFiles) => {
        const validFiles = Array.from(selectedFiles).filter(file =>
            isValidFileType(file)
        );

        // 数量限制逻辑
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
        generatePreviews(filteredFiles);
    };

    // 拖放处理
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

    // 上传 - 使用批量上传API
    const handleUpload = async () => {
        if (files.length === 0) {
            setError('Please select photos to upload');
            setTimeout(() => setError(''), 3000);
            return;
        }

        try {
            setUploadProgress(0);
            
            // 创建FormData对象用于批量上传
            const formData = new FormData();
            
            // 添加所有文件到formData，使用相同的字段名'files'
            files.forEach(file => {
                formData.append('files', file);
            });
            
            // 调用批量上传API
            const response = await fetch(`/api/photos/batch`, {
                method: 'POST',
                body: formData,
            });
            
            if (!response.ok) {
                throw new Error('Batch upload failed');
            }
            
            const result = await response.json();
            
            // 设置进度为100%表示完成
            setUploadProgress(100);
            
            // 显示成功消息，包含成功上传的数量
            console.log(result)
            setSuccess(`Successfully uploaded ${result.data.successful} of ${result.data.total} photos!`);

            // 清理状态
            setTimeout(() => {
                setSuccess('');
                setFiles([]);
                setPreviews([]);
                setUploadProgress(0);
            }, 2000);
        } catch (err) {
            setError(err.message || 'Upload failed, please try again');
            setTimeout(() => setError(''), 3000);
        }
    }

    return (
        <div className="min-h-screen px-2">
            <div className="max-w-3xl mx-auto">
                <h1 className="text-3xl font-bold mb-8">Upload Photos</h1>
                <small className="text-sm text-base-content/70 mb-4">
                    This page is for temporary upload, if you want to upload more photos at once,
                    please directly change the directory in the file system.
                </small>

                {/* 拖放区域 */}
                <div
                    className={`border-2 border-dashed rounded-xl p-8 mb-6 text-center transition-colors
                                ${isDragging ? 'border-blue-500 bg-blue-50' : 'border-gray-300 hover:border-blue-400'}`}
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    onDrop={handleDrop}
                    onClick={() => fileInputRef.current.click()}
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
                            <p className="font-medium">Drag or Click Here to Upload</p>
                            <p className="text-sm">Supports JPEG, PNG, RAW</p>
                        </div>
                    </div>
                </div>

                {/* 隐藏的文件输入 */}
                <input
                    type="file"
                    ref={fileInputRef}
                    className="hidden"
                    multiple
                    accept="image/*,
                          video/*,
                          .cr2, .nef, .arw, .raf, .rw2, .dng,
                          .mov, .mp4, .avi, .mkv"
                    onChange={(e) => handleFiles(e.target.files)}
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
                    <div className="mb-4">
                        <div className="flex items-center gap-2">
                            <progress
                                className="progress w-56"
                                value={thumbnailProgress.numberProcessed}
                                max={thumbnailProgress.total}
                            ></progress>
                            <span className="text-sm text-gray-500">
                            {thumbnailProgress.numberProcessed}/{thumbnailProgress.total}
                        </span>
                        </div>
                    </div>
                )}

                {/* Loading indicator when generating previews */}
                {isGeneratingThumbnails && (
                    <div className="flex justify-center items-center mb-6">
                        <span className="loading loading-dots loading-md"></span>
                        <span className="ml-2 text-sm text-gray-500">Generating previews...</span>
                    </div>
                )}

                {/* 预览区域 */}
                {previews.length > 0 && (
                    <div className="grid grid-cols-5 gap-4 mb-6">
                        {previews.map((url, index) => (
                            <div
                                key={index}
                                className="aspect-square bg-gray-100 rounded-lg overflow-hidden shadow-sm"
                            >
                                {url ? (
                                    <img
                                        src={url}
                                        alt={`preview ${index + 1}`}
                                        className="w-full h-full object-cover"
                                    />
                                ) : (
                                    <div className="skeleton h-full w-full"></div>
                                )}
                            </div>
                        ))}
                    </div>
                )}

                {/* 进度条 */}
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


                {/* 操作按钮 */}
                <div className="flex justify-end gap-4">
                    <button
                        onClick={() => {
                            setFiles([]);
                            setPreviews([]);
                            setUploadProgress(0);
                        }}
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

                {/* 状态提示 */}
                {error && (
                    <div className="toast toast-top toast-right">
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


                {!wasmReady && (
                    <div className="text-xs text-amber-500 mt-1">
                        WebAssembly module is loading...
                    </div>
                )}
            </div>
        </div>
    );
};

export default UploadPhotos;