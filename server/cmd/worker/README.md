# 任务流程定义
任务（去歧义）是指用户派发的，由API识别的任务。

## 照片上传 Pipeline
`BatchUpload/Upload`
WAL任务派发
1.UPLOAD → 2. PROCESS → 3. INDEX

## 照片扫描 Pipeline
`Scan`
WAL任务派发
1.SCAN → 2. PROCESS → 3. INDEX
