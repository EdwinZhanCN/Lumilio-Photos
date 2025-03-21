import { useState, useRef, useCallback } from 'react';

const UploadPhotos = () => {
    const [files, setFiles] = useState([]);
    const [previews, setPreviews] = useState([]);
    const [isDragging, setIsDragging] = useState(false);
    const [progress, setProgress] = useState(0);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const fileInputRef = useRef(null);
    const [maxFiles] = useState(30);
    const [isGeneratingPreview, setIsGeneratingPreview] = useState(false);
    const rawFileExtensions = ['.raw', '.cr2', '.nef', '.orf', '.sr2',
        '.arw', '.rw2', '.dng', '.k25', '.kdc', '.mrw', '.pef', '.raf', '.3fr', '.fff'];
    // 清理所有生成的URL
    const revokePreviews = useCallback((urls) => {
        urls.forEach(url => URL.revokeObjectURL(url));
    }, []);


    // 生成预览图
    const generatePreviews = useCallback(async (files) => {
        setIsGeneratingPreview(true);
        const startIndex = previews.length;

        // 初始化占位数组
        setPreviews(prev => [...prev, ...Array(files.length).fill(null)]);

        // RAW文件配置
        const rawConfig = {
            mimeTypes: [
                'image/x-canon-cr2',
                'image/x-nikon-nef',
                'image/x-sony-arw',
                'image/x-adobe-dng',
                'image/x-fuji-raf',
                'image/x-panasonic-rw2'
            ],
            extensions: ['cr2', 'nef', 'arw', 'raf', 'rw2', 'dng', 'cr3', '3fr', 'orf']
        };

        // 生成视频预览
        const createVideoPreview = () => {
            const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
        <path fill="#ccc" d="M16 16c0 1.104-.896 2-2 2H4c-1.104 0-2-.896-2-2V8c0-1.104.896-2 2-2h10c1.104 0 2 .896 2 2v8zm4-10h-2v2h2v8h-2v2h4V6z"/>
      </svg>`;
            return URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
        };

        // 生成RAW预览
        const createRawPreview = (file) => {
            const extension = file.name.split('.').pop().toUpperCase();
            const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120" width="120" height="120">
        <rect width="100%" height="100%" fill="#e2e8f0" rx="8" ry="8"/>
        <text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle"
              font-family="system-ui, -apple-system, sans-serif"
              font-weight="600"
              fill="#475569">
          <tspan x="50%" dy="-0.6em" font-size="14">RAW</tspan>
          <tspan x="50%" dy="1.8em" font-size="12">${extension}</tspan>
        </text>
      </svg>`;
            return URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
        };

        // 压缩图片（使用OffscreenCanvas提升性能）
        const compressImage = async (file) => {
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
        };

        // 分批次处理
        const BATCH_SIZE = 4;
        try {
            for (let i = 0; i < files.length; i += BATCH_SIZE) {
                const batch = files.slice(i, i + BATCH_SIZE);

                const batchPromises = batch.map((file, batchIndex) => {
                    // 视频文件
                    if (file.type.startsWith('video/')) {
                        return createVideoPreview();
                    }

                    // RAW文件
                    if (rawConfig.mimeTypes.includes(file.type) ||
                        rawConfig.extensions.includes(file.name.split('.').pop().toLowerCase())) {
                        return createRawPreview(file);
                    }

                    // 普通图片
                    return compressImage(file);
                });

                const processedUrls = await Promise.all(batchPromises);

                // 更新对应位置的预览
                setPreviews(prev => {
                    const newPreviews = [...prev];
                    processedUrls.forEach((url, index) => {
                        newPreviews[startIndex + i + index] = url;
                    });
                    return newPreviews;
                });
            }
        } catch (error) {
            console.error('Preview generation error:', error);
            revokePreviews(previews);
            setPreviews([]);
        } finally {
            setIsGeneratingPreview(false);
        }
    }, [previews, revokePreviews]);



    // 处理文件选择
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
            setProgress(0);
            
            // 创建FormData对象用于批量上传
            const formData = new FormData();
            
            // 添加所有文件到formData，使用相同的字段名'files'
            files.forEach(file => {
                formData.append('files', file);
            });
            
            // 调用批量上传API
            const response = await fetch('http://localhost:3001/api/photos/batch', {
                method: 'POST',
                body: formData,
            });
            
            if (!response.ok) {
                throw new Error('Batch upload failed');
            }
            
            const result = await response.json();
            
            // 设置进度为100%表示完成
            setProgress(100);
            
            // 显示成功消息，包含成功上传的数量
            console.log(result)
            setSuccess(`Successfully uploaded ${result.data.successful} of ${result.data.total} photos!`);

            // 清理状态
            setTimeout(() => {
                setSuccess('');
                setFiles([]);
                setPreviews([]);
                setProgress(0);
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

                {/* 预览区域 */}
                {previews.length > 0 && (
                    <div className="grid grid-cols-5 gap-4 mb-6">
                        {previews.map((url, index) => (
                            <div
                                key={index}
                                className="aspect-square bg-gray-100 rounded-lg overflow-hidden shadow-sm"
                            >
                                <img
                                    src={url}
                                    alt={`preview ${index + 1}`}
                                    className="w-full h-full object-cover"
                                />
                            </div>
                        ))}
                    </div>
                )}

                {/* Loading indicator when generating previews */}
                {isGeneratingPreview && (
                    <div className="flex justify-center items-center mb-6">
                        <span className="loading loading-dots loading-md"></span>
                        <span className="ml-2 text-sm text-gray-500">Generating previews...</span>
                    </div>
                )}

                {/* 进度条 */}
                {progress > 0 && (
                    <div className="mb-4">
                        <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                            <div
                                className="h-full bg-blue-500 transition-all duration-300"
                                style={{ width: `${progress}%` }}
                            />
                        </div>
                        <p className="text-sm text-gray-600 mt-1">
                            Uploading... {Math.min(progress, 99)}%
                        </p>
                    </div>
                )}

                {/* 操作按钮 */}
                <div className="flex justify-end gap-4">
                    <button
                        onClick={() => {
                            setFiles([]);
                            setPreviews([]);
                            setProgress(0);
                        }}
                        className="px-4 py-2 text-base-content/50 hover:text-base-content disabled:opacity-50"
                        disabled={files.length === 0 || progress > 0}
                    >
                        Clear
                    </button>
                    <button
                        onClick={handleUpload}
                        className="px-6 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 disabled:opacity-50 transition-colors
                        hover:cursor-pointer disabled:cursor-not-allowed"
                        disabled={files.length === 0 || progress > 0}
                    >
                        {progress > 0 ? 'Uploading...' : 'Start Upload'}
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
            </div>
        </div>
    );
};

export default UploadPhotos;