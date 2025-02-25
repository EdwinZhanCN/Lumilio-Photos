import React from "react";

// 示例照片数据（已扩展字段以便分组）
const photos = [
    {
        id: 1,
        date: "2024-01-10",
        name: "Sunset by the beach",
        size: 4096, // 以字节为单位
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

// 分组函数（可按需扩展）
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

const groupPhotosByName = (photos) => {
    return photos.reduce((acc, photo) => {
        const name = photo.name;
        if (!acc[name]) {
            acc[name] = [];
        }
        acc[name].push(photo);
        return acc;
    }, {});
};

const groupPhotosBySize = (photos) => {
    return photos.reduce((acc, photo) => {
        const size = photo.size;
        if (!acc[size]) {
            acc[size] = [];
        }
        acc[size].push(photo);
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

// 渲染组件
const Photos = () => {
    // 按日期分组照片数据
    const groupedPhotos = groupPhotosByDate(photos);

    return (
        <div className="p-4 w-full max-w-screen-lg mx-auto">
            {/* 根据日期渲染照片板块 */}
            {Object.keys(groupedPhotos).map((date) => (
                <div key={date} className="my-6">
                    {/* 日期标题 */}
                    <h2 className="text-xl font-bold mb-4 text-left">{date}</h2>
                    {/* 瀑布流布局的图片 */}
                    <div className="grid gap-2 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                        {groupedPhotos[date].map((photo) => (
                            <div key={photo.id} className="flex justify-center shadow-2xs hover:bg-base-300 hover:border-4 cursor-pointer transition-bg-shadow-border duration-300">
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