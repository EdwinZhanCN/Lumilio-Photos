import React, { useEffect, useRef } from 'react';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import 'leaflet/dist/leaflet.css';
import L from 'leaflet';

// 修复Leaflet默认图标问题
delete L.Icon.Default.prototype._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.7.1/dist/images/marker-icon-2x.png',
  iconUrl: 'https://unpkg.com/leaflet@1.7.1/dist/images/marker-icon.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.7.1/dist/images/marker-shadow.png',
});

function MapComponent({ photoLocations = [] }) {
  // 如果没有照片位置数据，使用默认位置（北京）
  const defaultCenter = [39.9042, 116.4074]; // 北京坐标
  const defaultZoom = 5;
  
  // 示例照片位置数据
  const sampleLocations = [
    { id: 1, position: [39.9042, 116.4074], title: '北京', description: '天安门广场' },
    { id: 2, position: [31.2304, 121.4737], title: '上海', description: '外滩' },
    { id: 3, position: [22.5431, 114.0579], title: '深圳', description: '深圳湾' },
    { id: 4, position: [30.5728, 104.0668], title: '成都', description: '锦里古街' },
  ];
  
  // 使用传入的照片位置或示例数据
  const locations = photoLocations.length > 0 ? photoLocations : sampleLocations;
  
  return (
    <MapContainer 
      center={defaultCenter} 
      zoom={defaultZoom} 
      style={{ height: '100%', width: '100%' }}
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      
      {locations.map((location) => (
        <Marker key={location.id} position={location.position}>
          <Popup>
            <div>
              <h3 className="font-bold">{location.title}</h3>
              <p>{location.description}</p>
            </div>
          </Popup>
        </Marker>
      ))}
    </MapContainer>
  );
}

export default MapComponent;