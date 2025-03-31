import React, { useState } from 'react';
import { ExifInfo } from '../utils/exifInfo';
import { SparklesIcon, MapIcon, ClockIcon, CameraIcon, SwatchIcon, GlobeAltIcon } from '@heroicons/react/24/outline';
import MapComponent from '../components/MapComponent';
import Heatmap from 'react-calendar-heatmap';
import '@/styles/heatmap.css';

function Home() {
    const [hoverIndex, setHoverIndex] = useState(null);
    const [displayMode, setDisplayMode] = useState('gallery'); // 新增状态管理
    
    return (
        <div className="flex flex-col gap-8 p-6 relative">
            {/* 玻璃拟态风格Tab切换 */}
            <div className="tabs tabs-boxed bg-base-100/20 backdrop-blur-lg rounded-box p-1 absolute top-4 right-4 z-10 shadow-lg">
                <a 
                    className={`tab tab-lg rounded-box p-1 m-1 ${displayMode === 'gallery' ? 'tab-active bg-primary/20 text-primary' : ''}`}
                    onClick={() => setDisplayMode('gallery')}
                >
                    <SparklesIcon className="size-5 mr-2" />
                    画廊模式
                </a> 
                <a 
                    className={`tab tab-lg rounded-box p-1 m-1 ${displayMode === 'stats' ? 'tab-active bg-primary/20 text-primary' : ''}`}
                    onClick={() => setDisplayMode('stats')}
                >
                    <CameraIcon className="size-5 mr-2" />
                    数据统计
                </a>
            </div>

            {/* 画廊模式内容 */}
            {displayMode === 'gallery' && (
                <>
                    {/* 3D照片瀑布流 */}
                    <section className="min-h-[60vh] relative group">
                        <div className="absolute inset-0 bg-gradient-to-b from-primary/10 to-transparent rounded-3xl" />
                        <div className="m-3 grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 transform group-hover:scale-[0.98] transition-transform">
                            {[...Array(8)].map((_, i) => (
                                <div 
                                    key={i}
                                    className="aspect-square bg-base-200 rounded-2xl shadow-lg hover:shadow-2xl transition-all
                                           hover:-translate-y-2 cursor-zoom-in relative overflow-hidden"
                                    onMouseEnter={() => setHoverIndex(i)}
                                    onMouseLeave={() => setHoverIndex(null)}
                                >
                                    <div className="absolute inset-0 bg-gradient-to-t from-black/30 to-transparent" />
                                    {hoverIndex === i && (
                                        <div className="absolute inset-0 p-3 bg-black/80 backdrop-blur-sm flex flex-col justify-center">
                                            <div className="text-xs text-primary mb-2">摄影参数</div>
                                            <div className="text-[11px] space-y-1 text-white/80">
                                                <div>📷 {ExifInfo.getSample().camera}</div>
                                                <div>🔍 {ExifInfo.getSample().lens}</div>
                                                <div>⭕ ƒ/{ExifInfo.getSample().aperture}</div>
                                                <div>⏱️ {ExifInfo.getSample().shutter}</div>
                                                <div>📏 {ExifInfo.getSample().focalLength}</div>
                                                <div>✨ {ExifInfo.getSample().iso}</div>
                                            </div>
                                        </div>
                                    )}
                                    <div className="absolute bottom-2 left-2 text-white text-sm">
                                        示例照片 {i + 1}
                                    </div>
                                </div>
                            ))}
                        </div>
                        <SparklesIcon className="absolute top-4 right-4 size-8 text-primary animate-pulse" />
                    </section>

                    {/* AI分类展示墙 */}
                    <section className="carousel carousel-center gap-4 p-4 bg-base-200 rounded-3xl">
                        {['旅行记忆', '家庭时光', '自然景观', '美食记录'].map((cat, i) => (
                            <div key={cat} className="carousel-item">
                                <div className="card w-64 bg-base-100 shadow-xl hover:shadow-2xl transition-shadow">
                                    <figure className="aspect-video bg-base-200 animate-pulse" />
                                    <div className="card-body">
                                        <h3 className="card-title">{cat}</h3>
                                        <p className="text-sm opacity-70">AI识别到{(i+1)*12}个相关项目</p>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </section>

                    {/* 创意滤镜预览墙 */}
                    <section className="carousel carousel-center gap-4 p-4 bg-base-200 rounded-3xl">
                        {['胶片模拟', '赛博朋克', '复古褪色', '黑金夜景'].map((filter, i) => (
                            <div key={filter} className="carousel-item">
                                <div className="group relative w-48 h-64 bg-base-100 rounded-xl shadow-lg hover:shadow-2xl transition-all">
                                    <div className="absolute inset-0 bg-gradient-to-t from-black/40 to-transparent rounded-xl" />
                                    <div className="absolute bottom-3 left-3 text-white text-sm">{filter}</div>
                                    <div className="absolute top-3 right-3">
                                        <SwatchIcon className="size-5 text-primary/80" />
                                    </div>
                                    <button className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity
                                               bg-black/50 backdrop-blur-sm flex items-center justify-center">
                                        <span className="btn btn-xs btn-primary">预览效果</span>
                                    </button>
                                </div>
                            </div>
                        ))}
                    </section>
                </>
            )}

            {/* 统计模式内容 */}
            {displayMode === 'stats' && (
                <div className="space-y-8 animate-fadeIn">
                    {/* 摄影参数统计卡片组 */}
                    <section className="grid grid-cols-1 md:grid-cols-2 gap-4 p-4 bg-base-200 rounded-3xl">
                        <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
                            <div className="card-body">
                                <div className="flex items-center gap-2 text-primary">
                                    <CameraIcon className="size-5" />
                                    <h3 className="font-bold">常用焦段分布</h3>
                                </div>
                                <div className="text-sm space-y-2 mt-2">
                                    <div className="flex justify-between">
                                        <span>24mm</span>
                                        <span className="text-primary">35%</span>
                                    </div>
                                    <progress className="progress progress-primary w-full" value="35" max="100" />
                                </div>
                            </div>
                        </div>
                        <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
                            <div className="card-body">
                                <div className="flex items-center gap-2 text-primary">
                                    <ClockIcon className="size-5" />
                                    <h3 className="font-bold">拍摄时段分布</h3>
                                </div>
                                <div className="radial-progress text-primary mt-2" 
                                     style={{ '--value': 70, '--size': '3rem' }}>
                                    70%
                                </div>
                                <p className="text-sm mt-2">黄金时段拍摄占比</p>
                            </div>
                        </div>
                        <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
                            <div className="card-body">
                                <div className="flex items-center gap-2 text-primary">
                                    <CameraIcon className="size-5" />
                                    <h3 className="font-bold">常用相机镜头组合</h3>
                                </div>
                                <div className="text-sm space-y-2 mt-2">
                                    {[
                                        { combo: 'Canon EOS R5 + RF24-70mm', rate: 45 },
                                        { combo: 'Sony A7IV + FE 24-70mm GM', rate: 30 },
                                        { combo: 'Fujifilm X-T4 + XF16-55mm', rate: 15 },
                                        { combo: 'Nikon Z7II + Z 24-70mm', rate: 8 },
                                        { combo: 'Leica Q3 + Summilux 28mm', rate: 2 }
                                    ].map((item, i) => (
                                        <div key={i} className="space-y-1">
                                            <div className="flex justify-between text-xs">
                                                <span>{item.combo}</span>
                                                <span className="text-primary">{item.rate}%</span>
                                            </div>
                                            <progress 
                                            className="progress progress-primary w-full h-1" 
                                            value={item.rate} 
                                            max="100" 
                                            />
                                        </div>
                                    ))}
                                </div>
                            </div>
                        </div>
                        <div className="card bg-base-100 shadow-sm hover:shadow-md transition-shadow">
                            <div className="card-body">
                                <div className="flex items-center gap-2 text-primary">
                                    <ClockIcon className="size-5" />
                                    <h3 className="font-bold">拍摄活跃热力图</h3>
                                </div>
                                <div className="mt-2">
                                    <Heatmap
                                        values={generateSampleHeatmapData()}
                                        classForValue={(value) => {
                                            if (!value) return 'color-empty';
                                            return `color-scale-${Math.min(value.count, 5)}`;
                                        }}
                                        tooltipDataAttrs={(value) => ({
                                            'data-tip': `${value.date} 拍摄了 ${value.count} 张`,
                                        })}
                                    />
                                </div>
                            </div>
                        </div>
                    </section>



                    {/* 摄影网站资讯入口 */}
                    <section className="card bg-primary text-primary-content shadow-xl">
                        <div className="card-body">
                            <h2 className="card-title">
                                <GlobeAltIcon className="size-6" />
                                摄影网站资讯
                            </h2>
                            <p>发现500px、Flickr等平台的优秀作品与摄影技巧分享</p>
                            <div className="card-actions justify-end">
                                <button className="btn btn-secondary">浏览资讯</button>
                            </div>
                        </div>
                    </section>
                </div>
            )}

            {/* 时空地图整合区（始终显示） */}
            <section className="card bg-base-100 shadow-xl overflow-hidden">
                <div className="card-body p-0">
                    <div className="flex items-center bg-base-200 p-4">
                        <MapIcon className="size-6 mr-2 text-primary" />
                        <h2 className="text-xl font-bold">时空轨迹</h2>
                    </div>
                    <div className="aspect-[16/9]">
                        <MapComponent />
                    </div>
                </div>
            </section>
        </div>
    );
}

export default Home;

// 生成示例热力图数据
const generateSampleHeatmapData = () => {
  const startDate = new Date('2024-01-01');
  const endDate = new Date('2024-12-31');
  const data = [];
  let currentDate = new Date(startDate);

  while (currentDate <= endDate) {
    const count = Math.floor(Math.random() * 5);
    if (Math.random() > 0.7) {
      data.push({
        date: currentDate.toISOString().split('T')[0],
        count: count
      });
    }
    currentDate.setDate(currentDate.getDate() + 1);
  }
  return data;
};