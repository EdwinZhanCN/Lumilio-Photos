-- name: GetFocalLengthDistribution :many
-- 获取焦距分布统计
SELECT
    (specific_metadata->>'focal_length')::numeric AS focal_length,
    COUNT(*) AS count
FROM assets
WHERE
    is_deleted = false
    AND specific_metadata->>'focal_length' IS NOT NULL
    AND specific_metadata->>'focal_length' != ''
    AND (specific_metadata->>'focal_length')::numeric > 0
GROUP BY focal_length
ORDER BY count DESC
LIMIT 50;

-- name: GetCameraLensStats :many
-- 获取相机+镜头组合统计
SELECT
    specific_metadata->>'camera_model' AS camera_model,
    specific_metadata->>'lens_model' AS lens_model,
    COUNT(*) AS count
FROM assets
WHERE
    is_deleted = false
    AND specific_metadata->>'camera_model' IS NOT NULL
    AND specific_metadata->>'camera_model' != ''
    AND specific_metadata->>'lens_model' IS NOT NULL
    AND specific_metadata->>'lens_model' != ''
GROUP BY camera_model, lens_model
ORDER BY count DESC
LIMIT $1;

-- name: GetTimeDistributionHourly :many
-- 获取按小时的拍摄时间分布
SELECT
    EXTRACT(HOUR FROM COALESCE(taken_time, upload_time))::integer AS hour,
    COUNT(*) AS count
FROM assets
WHERE
    is_deleted = false
    AND (taken_time IS NOT NULL OR upload_time IS NOT NULL)
GROUP BY hour
ORDER BY hour;

-- name: GetTimeDistributionMonthly :many
-- 获取按月的拍摄时间分布
SELECT
    DATE_TRUNC('month', COALESCE(taken_time, upload_time))::timestamp AS month,
    COUNT(*) AS count
FROM assets
WHERE
    is_deleted = false
    AND (taken_time IS NOT NULL OR upload_time IS NOT NULL)
GROUP BY month
ORDER BY month DESC
LIMIT 24;

-- name: GetDailyActivityHeatmap :many
-- 获取每日拍摄活跃度热力图数据
SELECT
    DATE(COALESCE(taken_time, upload_time)) AS date,
    COUNT(*) AS count
FROM assets
WHERE
    is_deleted = false
    AND (taken_time IS NOT NULL OR upload_time IS NOT NULL)
    AND COALESCE(taken_time, upload_time) >= $1
    AND COALESCE(taken_time, upload_time) <= $2
GROUP BY date
ORDER BY date;

-- name: GetAvailableYears :many
-- 获取所有有照片的年份列表
SELECT DISTINCT
    EXTRACT(YEAR FROM COALESCE(taken_time, upload_time))::integer AS year
FROM assets
WHERE
    is_deleted = false
    AND (taken_time IS NOT NULL OR upload_time IS NOT NULL)
ORDER BY year DESC;
