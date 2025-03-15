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
    const rawFileExtensions = ['.raw', '.cr2', '.nef', '.orf', '.sr2',
        '.arw', '.rw2', '.dng', '.k25', '.kdc', '.mrw', '.pef', '.raf', '.3fr', '.fff'];

    // 生成预览图
    const generatePreviews = useCallback((files) => {
        const urls = [];
        const rawMimeTypes = [
            'image/x-canon-cr2',    // Canon
            'image/x-nikon-nef',    // Nikon
            'image/x-sony-arw',     // Sony
            'image/x-adobe-dng',    // Adobe
            'image/x-fuji-raf',     // Fujifilm
            'image/x-panasonic-rw2' // Panasonic
        ];
        const rawExtensions = ['cr2', 'nef', 'arw', 'raf', 'rw2', 'dng', 'cr3', '3fr', 'orf'];

        for (const file of files) {
            // 处理视频文件
            if (file.type.startsWith('video/')) {
                urls.push(URL.createObjectURL(new Blob(
                    ['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#ccc" d="M16 16c0 1.104-.896 2-2 2H4c-1.104 0-2-.896-2-2V8c0-1.104.896-2 2-2h10c1.104 0 2 .896 2 2v8zm4-10h-2v2h2v8h-2v2h4V6z"/></svg>'],
                    { type: 'image/svg+xml' }
                )));
            }
            // 处理RAW文件
            else if (
                rawMimeTypes.includes(file.type) ||
                rawExtensions.includes(file.name.split('.').pop().toLowerCase())
            ) {
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

                urls.push(URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' })));
            }
            // 处理普通图片
            else {
                urls.push(URL.createObjectURL(file));
            }
        }
        // 设置预览图, 旧的加新的
        setPreviews(prev => [...prev, ...urls]);
    }, []);



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

    // 模拟上传过程
    const handleUpload = async () => {
        if (files.length === 0) {
            setError('Please select photos to upload');
            setTimeout(() => setError(''), 3000);
            return;
        }

        try {
            setProgress(0);
            let uploadedCount = 0;

            for (const file of files) {
                const formData = new FormData();
                formData.append('file', file);

                const response = await fetch('http://localhost:3001/api/photos', {
                    method: 'POST',
                    body: formData,
                });

                if (!response.ok) {
                    throw new Error(`Upload failed for ${file.name}`);
                }

                uploadedCount++;
                setProgress((uploadedCount / files.length) * 100);
            }

            setSuccess('Photos uploaded successfully!');
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
    };

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
                        className="px-6 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 disabled:opacity-50 transition-colors"
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
                    <div className="toast toast-top toast-right">
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