import React, { useState } from "react";

const photos = [
    {
        id: 1,
        date: "2024-01-10",
        name: "Sunset by the beach",
        size: 409600000, // 以字节为单位
        tags: ["nature", "sunrise"],
        type: "JPEG",
        url: "https://picsum.photos/400/600",
        description: "Sample photo 1",
    },
    {
        id: 2,
        date: "2024-01-10",
        name: "City skyline at dusk",
        size: 2048,
        tags: ["city", "dusk"],
        type: "PNG",
        url: "https://picsum.photos/500/300",
        description: "Sample photo 2",
    },
    {
        id: 3,
        date: "2024-01-09",
        name: "Mountain trail",
        size: 3072,
        tags: ["nature", "mountains"],
        type: "JPEG",
        url: "https://picsum.photos/300/400",
        description: "Sample photo 3",
    },
    {
        id: 4,
        date: "2024-01-09",
        name: "Abstract art piece",
        size: 1536,
        tags: ["art", "abstract"],
        type: "GIF",
        url: "https://picsum.photos/600/200",
        description: "Sample photo 4",
    },
    {
        id: 5,
        date: "2024-01-08",
        name: "Flower in a vase",
        size: 2560,
        tags: ["flowers", "still-life"],
        type: "PNG",
        url: "https://picsum.photos/500/500",
        description: "Sample photo 5",
    },
];

// 分组函数：按日期、标签、类型和大小范围
const groupPhotosByDate = (photos) => {
    return photos.reduce((acc, photo) => {
        const date = photo.date;
        if (!acc[date]) {
            acc[date] = [];
        }
        acc[date].push(photo);
        return acc;
    }, {});
};

const groupPhotosByTags = (photos) => {
    return photos.reduce((acc, photo) => {
        photo.tags.forEach((tag) => {
            if (!acc[tag]) {
                acc[tag] = [];
            }
            acc[tag].push(photo);
        });
        return acc;
    }, {});
};

const groupPhotosByType = (photos) => {
    return photos.reduce((acc, photo) => {
        const type = photo.type;
        if (!acc[type]) {
            acc[type] = [];
        }
        acc[type].push(photo);
        return acc;
    }, {});
};

// 新增分组函数：按大小范围
const groupPhotosBySizeRange = (photos) => {
    // 定义大小分组
    const sizeGroups = [
        { name: ">1000MB", condition: (mb) => mb > 1000 },
        { name: "100MB - 1000MB", condition: (mb) => mb >= 100 && mb <= 1000 },
        { name: "1MB - 100MB", condition: (mb) => mb >= 1 && mb < 100 },
        { name: "<1MB", condition: (mb) => mb < 1 },
    ];

    return photos.reduce((acc, photo) => {
        const bytes = photo.size;
        if (bytes === 0) return acc; // 跳过大小为 0 的照片

        // 将字节转换为 MB
        const mb = bytes / 1048576; // 1MB = 1024^2 bytes

        // 寻找合适的分组
        const group = sizeGroups.find((g) => g.condition(mb));
        const groupKey = group ? group.name : "Unknown Size";

        if (!acc[groupKey]) {
            acc[groupKey] = [];
        }
        acc[groupKey].push(photo);

        return acc;
    }, {});
};

// 渲染组件
const Photos = () => {
    const [currentGrouping, setCurrentGrouping] = useState("date"); // 当前分组方式
    const [groupedPhotos, setGroupedPhotos] = useState({});
    const [isCarouselOpen, setIsCarouselOpen] = useState(false);
    const [currentPhotoIndex, setCurrentPhotoIndex] = useState(0);

    // 动态生成分组函数
    const groupingFunctions = {
        date: groupPhotosByDate,
        tags: groupPhotosByTags,
        type: groupPhotosByType,
        size: groupPhotosBySizeRange,
    };

    // 根据分组方式动态分组
    const getGroupedPhotos = () => {
        const groupingFunction = groupingFunctions[currentGrouping];
        if (groupingFunction) {
            const grouped = groupingFunction(photos);
            setGroupedPhotos(grouped);
        } else {
            setGroupedPhotos({});
        }
    };

    // 初始化和更新分组
    React.useEffect(() => {
        getGroupedPhotos();
    }, [currentGrouping]);

    // 初始化分组方式为按日期
    React.useEffect(() => {
        setCurrentGrouping("date");
    }, []);

    return (
        <div className="p-4 w-full max-w-screen-lg mx-auto">
            {/* 顶部栏：标题和下拉菜单 */}
            <div className="flex gap-2 items-center mb-4">
                <h1 className="text-2xl font-bold">Photos</h1>
                {/* 下拉菜单 */}
                <div className="dropdown">
                    <div
                        tabIndex={0}
                        role="button"
                        className="btn btn-ghost m-1"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-5">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5M12 17.25h8.25" />
                        </svg>
                    </div>
                    <ul
                        tabIndex={0}
                        className="dropdown-content menu bg-base-200 rounded-box z-1 w-52 p-2 shadow-sm"
                    >
                        {/* 下拉菜单选项 */}
                        <li>
                            <a
                                onClick={() => setCurrentGrouping("date")}
                            >
                                Sort By Date
                            </a>
                        </li>
                        <li>
                            <a
                                onClick={() => setCurrentGrouping("size")}
                            >
                                Sort By Size Range
                            </a>
                        </li>
                        <li>
                            <a
                                onClick={() => setCurrentGrouping("tags")}
                            >
                                Sort By Tags
                            </a>
                        </li>
                        <li>
                            <a
                                onClick={() => setCurrentGrouping("type")}
                            >
                                Sort By Type
                            </a>
                        </li>
                    </ul>
                </div>
            </div>

            {/* 根据分组方式渲染照片板块 */}
            {Object.keys(groupedPhotos).map((groupKey) => (
                <div key={groupKey} className="my-6">
                    {/* 分组标题 */}
                    <h2 className="text-xl font-bold mb-4 text-left">
                        {({
                            date: "Date: ",
                            tags: "Tag: ",
                            type: "Type: ",
                            size: "Size Range: ",
                        })[currentGrouping]}
                        {groupKey}
                    </h2>

                    <div className="columns-1 sm:columns-2 lg:columns-3 xl:columns-4 gap-1">
                        {groupedPhotos[groupKey].map((photo) => (
                            <div
                                key={photo.id}
                                className="break-inside-avoid hover:bg-base-300 cursor-pointer transition-all"
                                onClick={() => {
                                    // 计算照片在全局数组中的索引
                                    const allPhotos = Object.values(groupedPhotos).flat();
                                    const currentIndex = allPhotos.findIndex((p) => p.id === photo.id);
                                    setCurrentPhotoIndex(currentIndex);
                                    setIsCarouselOpen(true);
                                }}
                            >
                                <img
                                    src={photo.url}
                                    alt={photo.description}
                                    className="w-full h-auto"
                                />
                            </div>
                        ))}
                    </div>
                </div>
            ))}

            {/* 全屏轮播组件 */}
            {isCarouselOpen && (
                <FullScreenCarousel
                    photos={Object.values(groupedPhotos).flat()}
                    initialSlide={currentPhotoIndex}
                />
            )}
        </div>
    );
};


// 全屏照片轮播组件
const FullScreenCarousel = ({ photos, initialSlide }) => {
    const [currentSlide, setCurrentSlide] = useState(initialSlide);
    const totalSlides = photos.length;

    // 翻页功能
    const handlePrev = () => {
        setCurrentSlide((prev) => (prev === 0 ? totalSlides - 1 : prev - 1));
    };

    const handleNext = () => {
        setCurrentSlide((prev) => (prev === totalSlides - 1 ? 0 : prev + 1));
    };

    const [showPhotoInfo, setShowPhotoInfo] = useState(false);
    const [selectedPhoto, setSelectedPhoto] = useState(null);

    //hide toolbar and arrows, show only image
    const handleHide = () => {
        const controls = document.getElementById("controls");
        const toolbar = document.getElementById("toolbar");

        // Add transition classes
        controls.classList.add("opacity-0", "transition-opacity", "duration-300", "ease-out");
        toolbar.classList.add("opacity-0", "transition-opacity", "duration-300", "ease-out");

        // Remove elements after animation completes
        setTimeout(() => {
            controls.style.display = "none";
            toolbar.style.display = "none";
        }, 300);
    };

    const handleShow = () => {
        const controls = document.getElementById("controls");
        const toolbar = document.getElementById("toolbar");

        // Reset display property
        controls.style.display = "flex";
        toolbar.style.display = "flex";

        // Force a reflow
        controls.offsetHeight;
        toolbar.offsetHeight;

        // Remove opacity-0 class to trigger fade-in
        controls.classList.remove("opacity-0");
        toolbar.classList.remove("opacity-0");
    };



    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-base-300 bg-opacity-90 overflow-hidden">
            {/* 遮罩层，点击可关闭轮播 */}
            <div
                className="absolute inset-0 bg-base-300 bg-opacity-90"
                onClick={(e) => {
                    if (e.target === e.currentTarget) {
                        console.log("Close carousel");
                        // 您可以在这里实现关闭轮播的功能，例如使用全局状态或父级 props
                    }
                }}
            />

            {/* 全屏图片 */}
            <div className="relative w-full h-full">
                <img
                    src={photos[currentSlide].url}
                    alt={photos[currentSlide].description}
                    className="absolute inset-0 w-full h-full object-contain"
                    onClick={handleShow}
                />
            </div>

            {/* 左右箭头 */}
            <div id="controls" className="absolute inset-0 flex items-center justify-between px-4">
                <button
                    onClick={handlePrev}
                    className="btn-circle p-4 cursor-pointer bg-black/30 backdrop-blur-sm"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-6">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
                    </svg>
                </button>
                <button
                    onClick={handleNext}
                    className="btn-circle p-4 cursor-pointer bg-black/30 backdrop-blur-sm"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-6">
                        <path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
                    </svg>
                </button>
            </div>

            {/* 展示照片信息 */}
            {showPhotoInfo && selectedPhoto && (
                <PhotoInfo photo={selectedPhoto} onClose={() => setShowPhotoInfo(false)} />
            )}

            {/* 悬浮工具bar */}
            <div id="toolbar" className="flex absolute gap-2 justify-between items-center bg-gray-800/30 backdrop-blur-sm bottom-1/8 p-3 rounded-full">
                <div>
                    <button
                        onClick={() => {
                            handleHide();
                        }}
                        className="btn-circle p-2 cursor-pointer bg-black/30 dark:bg-white/30 backdrop-blur-sm hover:bg-black/40 dark:hover:bg-white/40"
                        >
                        <svg className="size-6" viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg">
                            <path fill="#000000" d="m160 96.064 192 .192a32 32 0 0 1 0 64l-192-.192V352a32 32 0 0 1-64 0V96h64v.064zm0 831.872V928H96V672a32 32 0 1 1 64 0v191.936l192-.192a32 32 0 1 1 0 64l-192 .192zM864 96.064V96h64v256a32 32 0 1 1-64 0V160.064l-192 .192a32 32 0 1 1 0-64l192-.192zm0 831.872-192-.192a32 32 0 0 1 0-64l192 .192V672a32 32 0 1 1 64 0v256h-64v-.064z"/>
                        </svg>
                    </button>
                </div>
                <div>
                    <button
                        onClick={() => {
                            console.log("Share Photo");
                            // 您可以在这里实现图片分享的功能，例如使用浏览器 API
                        }}
                        className="btn-circle p-2 cursor-pointer bg-black/30 dark:bg-white/30 backdrop-blur-sm hover:bg-black/40 dark:hover:bg-white/40"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-6">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0 0a2.25 2.25 0 1 0 3.935 2.186 2.25 2.25 0 0 0-3.935-2.186Zm0-12.814a2.25 2.25 0 1 0 3.933-2.185 2.25 2.25 0 0 0-3.933 2.185Z" />
                        </svg>
                    </button>
                </div>
                <div>
                    <button
                        onClick={() => {
                            console.log("Like Photo");
                            // 您可以在这里实现图片点赞的功能，例如使用浏览器 API
                        }}
                        className="btn-circle p-2 cursor-pointer bg-black/30 dark:bg-white/30 backdrop-blur-sm hover:bg-black/40 dark:hover:bg-white/40"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-6">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M21 8.25c0-2.485-2.099-4.5-4.688-4.5-1.935 0-3.597 1.126-4.312 2.733-.715-1.607-2.377-2.733-4.313-2.733C5.1 3.75 3 5.765 3 8.25c0 7.22 9 12 9 12s9-4.78 9-12Z" />
                        </svg>
                    </button>
                </div>
                <div>
                    <input
                        type="checkbox"
                        value="photoInfo"
                        id="photoInfo"
                        checked={showPhotoInfo}
                        onChange={(e) => {
                            if (e.target.checked) {
                                setSelectedPhoto(photos[currentSlide]); // Use current photo from carousel
                                setShowPhotoInfo(true);
                            } else {
                                setShowPhotoInfo(false);
                            }
                        }}
                        className="hidden"
                    />
                    <label htmlFor="photoInfo">
                        <div className="btn-circle p-2 cursor-pointer bg-black/30 dark:bg-white/30 backdrop-blur-sm hover:bg-black/40 dark:hover:bg-white/40">
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                fill="none"
                                viewBox="0 0 24 24"
                                strokeWidth={1.5}
                                stroke="currentColor"
                                className="size-6"
                            >
                                {/* SVG path */}
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z"
                                />
                            </svg>
                        </div>
                    </label>

                </div>
                {/* count */}
                <div className="text-white">
                    {currentSlide + 1} / {totalSlides}
                </div>
                <div className="dropdown dropdown-top dropdown-end">
                    <div tabIndex={0} role="button" className="btn-circle p-2 cursor-pointer bg-black/30 dark:bg-white/30 backdrop-blur-sm hover:bg-black/40 dark:hover:bg-white/40">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-6">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0ZM12.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0ZM18.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z" />
                        </svg>
                    </div>
                    <ul tabIndex={0} className="dropdown-content menu bg-base-100 rounded-box z-1 w-52 p-2 shadow-sm">
                        <li><a>Item 1</a></li>
                        <li><a>Item 2</a></li>
                    </ul>
                </div>
                <div>
                    <button
                        onClick={() => {
                            console.log("Delete Photo");
                            // 您可以在这里实现图片点赞的功能，例如使用浏览器 API
                        }}
                        className="btn-circle p-2 cursor-pointer text-error bg-black/30 dark:bg-white/30 backdrop-blur-sm hover:bg-black/40 dark:hover:bg-white/40"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-6">
                            <path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                        </svg>
                    </button>
                </div>
            </div>
        </div>
    );
};


const PhotoInfo = ({ photo, onClose }) => {
    return (
        <div
            className="fixed top-4 right-4 p-4 rounded-lg shadow-lg backdrop-blur backdrop-brightness-90 bg-base-100"
        >
            <div className="flex justify-between items-center">
                <h3 className="text-lg font-bold">{photo.name}</h3>
                <button
                    onClick={onClose}
                    className="text-gray-500 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-6">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                    </svg>
                </button>
            </div>
            <p className="text-sm mt-2">
                <strong>Date: </strong>
                {photo.date}
            </p>
            <p className="text-sm mt-1">
                <strong>Size: </strong>
                {photo.size} bytes
            </p>
            <p className="text-sm mt-1">
                <strong>Tags: </strong>
                {photo.tags.join(", ")}
            </p>
            <p className="text-sm mt-1">
                <strong>Type: </strong>
                {photo.type}
            </p>
            <p className="text-sm mt-1">{photo.description}</p>
        </div>
    );
};


export default Photos;