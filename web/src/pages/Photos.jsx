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

                    {/* 瀑布流布局的图片 */}
                    <div className="grid gap-2 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                        {groupedPhotos[groupKey].map((photo) => (
                            <div
                                key={photo.id}
                                className="flex justify-center shadow-2xs hover:bg-base-300 hover:border-4 cursor-pointer"
                            >
                                <img
                                    src={photo.url}
                                    alt={photo.description}
                                    className="w-full h-auto object-contain"
                                />
                            </div>
                        ))}
                    </div>
                </div>
            ))}
        </div>
    );
};

export default Photos;