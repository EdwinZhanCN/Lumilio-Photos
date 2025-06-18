# 存储策略配置指南

本文档介绍Lumilio照片管理系统支持的不同存储策略，帮助用户根据需求选择合适的文件组织方式。

## 支持的存储策略

### 1. 日期组织策略 (Date-based) - 默认推荐

**策略标识**: `date`  
**描述**: 按照上传时间的年份和月份组织文件  
**路径示例**: `2024/01/IMG_0001.jpg`

**优点**:
- 📁 直观易懂，用户容易通过SMB/FTP浏览
- 🕐 按时间线组织，方便查找特定时期的照片
- 🔍 支持文件管理器的日期筛选
- 👥 适合家庭用户和小团队

**缺点**:
- 📋 同名文件可能冲突（通过UUID后缀解决）
- 💾 无法自动去重

### 2. 内容寻址策略 (Content-Addressable Storage, CAS)

**策略标识**: `cas`  
**描述**: 基于文件内容的BLAKE3哈希值组织文件  
**路径示例**: `ab/cd/ef/abcdef123456789...jpg`

**优点**:
- 🔄 自动去重，相同文件只存储一份
- 🔒 内容完整性校验，防止文件损坏
- 💾 节省存储空间
- ⚡ 适合大规模部署和云存储

**缺点**:
- 📁 文件路径对用户不友好
- 🔧 需要应用层管理文件访问
- 📱 不适合直接通过SMB浏览

### 3. 平铺策略 (Flat)

**策略标识**: `flat`  
**描述**: 所有文件存储在同一目录中  
**路径示例**: `photo_uuid123.jpg`

**优点**:
- 🎯 简单直接的存储结构
- ⚡ 快速访问，无需遍历子目录

**缺点**:
- 📂 大量文件时目录性能下降
- 🔍 难以浏览和管理

## 配置方法

### 环境变量配置

```bash
# 基础存储路径
STORAGE_PATH=/app/data/photos

# 存储策略选择
STORAGE_STRATEGY=date        # date | cas | flat

# 是否保留原始文件名
STORAGE_PRESERVE_FILENAME=true

# 重名文件处理方式
STORAGE_DUPLICATE_HANDLING=rename  # rename | uuid | overwrite
```

### Docker Compose 配置示例

```yaml
version: '3.8'
services:
  lumilio-api:
    image: lumilio/api:latest
    environment:
      - STORAGE_PATH=/app/data/photos
      - STORAGE_STRATEGY=date                    # 推荐用户友好的日期策略
      - STORAGE_PRESERVE_FILENAME=true          # 保留原始文件名
      - STORAGE_DUPLICATE_HANDLING=rename       # 重名文件添加(1)(2)后缀
    volumes:
      - ./data/photos:/app/data/photos
  
  lumilio-worker:
    image: lumilio/worker:latest
    environment:
      - STORAGE_PATH=/app/data/photos
      - STORAGE_STRATEGY=date                    # 必须与API保持一致
      - STORAGE_PRESERVE_FILENAME=true
      - STORAGE_DUPLICATE_HANDLING=rename
    volumes:
      - ./data/photos:/app/data/photos
```

## 迁移策略

### 从CAS迁移到日期组织

如果当前使用CAS策略，想要迁移到对用户更友好的日期组织：

1. **停止服务**
```bash
docker-compose down
```

2. **备份数据**
```bash
cp -r ./data/photos ./data/photos-backup
```

3. **更新配置**
```bash
# 在.env文件中修改
STORAGE_STRATEGY=date
```

4. **运行迁移脚本** (未来实现)
```bash
# 计划中的迁移工具
./lumilio-migrate --from=cas --to=date
```

5. **重启服务**
```bash
docker-compose up -d
```

## 性能建议

### 小型家庭部署 (< 10万张照片)
```bash
STORAGE_STRATEGY=date
```
**原因**: 易于管理，SMB访问友好

### 中型团队部署 (10-100万张照片)
```bash
STORAGE_STRATEGY=date
```
**原因**: 仍然推荐日期策略，可通过增加子目录层级优化

### 大型企业部署 (> 100万张照片)
```bash
STORAGE_STRATEGY=cas
```
**原因**: 自动去重节省空间，适合API访问

## SMB/CIFS 挂载建议

### 日期策略的SMB配置

```bash
# 挂载命令
sudo mount -t cifs //nas-ip/photos /mnt/photos -o username=user,password=pass

# 目录结构
/mnt/photos/
├── 2024/
│   ├── 01/          # 一月份照片
│   ├── 02/          # 二月份照片
│   └── ...
├── 2023/
│   └── ...
```

### CAS策略的注意事项

CAS策略生成的路径如 `ab/cd/ef/abcdef123...jpg` 对普通用户不友好，建议：

1. 🔒 仅通过应用程序访问
2. 📱 使用Lumilio的Web界面或移动App
3. 🔗 通过API获取文件下载链接

## 故障排除

### 问题：找不到上传的文件

**解决方案**：
1. 检查`STORAGE_STRATEGY`环境变量
2. 确认API和Worker使用相同配置
3. 查看容器日志：`docker logs lumilio-api`

### 问题：文件重复存储

**解决方案**：
- 切换到CAS策略实现自动去重
- 配置重名处理方式：`STORAGE_DUPLICATE_HANDLING=rename`
- 或者在日期策略下手动清理重复文件

### 问题：同名文件被覆盖

**解决方案**：
- 设置 `STORAGE_DUPLICATE_HANDLING=rename` (默认推荐)
- 或设置 `STORAGE_DUPLICATE_HANDLING=uuid` 添加随机后缀
- 避免使用 `overwrite` 模式除非确实需要替换文件

### 问题：SMB访问慢

**解决方案**：
1. 避免使用CAS策略进行SMB访问
2. 优化Samba配置
3. 考虑使用NFS代替CIFS

## 文件命名与重名处理

### 重名文件处理方式

**1. rename模式 (推荐)**
```bash
STORAGE_DUPLICATE_HANDLING=rename
```
效果：
- `IMG_0001.jpg` → `IMG_0001.jpg`
- 再次上传同名文件 → `IMG_0001 (1).jpg`
- 第三次上传 → `IMG_0001 (2).jpg`

**2. uuid模式**
```bash
STORAGE_DUPLICATE_HANDLING=uuid
```
效果：
- `IMG_0001.jpg` → `IMG_0001.jpg`
- 再次上传同名文件 → `IMG_0001_a1b2c3d4.jpg`

**3. overwrite模式 (谨慎使用)**
```bash
STORAGE_DUPLICATE_HANDLING=overwrite
```
效果：
- 直接覆盖同名文件，可能导致数据丢失

### 文件名示例

**Date策略 + 原文件名保留**：
```
/app/data/photos/
├── 2024/
│   ├── 01/
│   │   ├── 新年快乐.jpg                ← 支持中文文件名
│   │   ├── IMG_0001.jpg               ← 相机原始文件名
│   │   ├── IMG_0001 (1).jpg           ← 重名文件自动编号
│   │   ├── vacation_beach.jpg          ← 用户自定义文件名
│   │   └── Screenshot_2024-01-15.png  ← 截图文件名
│   └── 02/
│       ├── birthday_party.jpg
│       └── family_dinner.mp4
```

## 最佳实践

1. **🏠 家庭用户**: 使用 `date` 策略 + `rename` 重名处理，便于通过文件管理器浏览
2. **💼 企业用户**: 根据规模选择，优先考虑 `date` 策略 + `uuid` 重名处理
3. **☁️ 云部署**: 使用 `cas` 策略节省存储成本，自动去重
4. **🔄 迁移规划**: 在系统规模扩大前规划存储策略
5. **📂 备份策略**: 无论使用哪种策略，都要定期备份
6. **📝 文件命名**: 保持原文件名便于用户识别和管理

## 未来计划

- [ ] 混合策略：新文件使用日期，自动检测重复文件使用CAS
- [ ] 智能迁移工具：支持策略间无缝切换
- [ ] 虚拟文件系统：为CAS策略提供友好的虚拟目录视图
- [ ] 用户自定义路径模板：如 `${year}/${month}/${camera-model}/`
