# 资源API
## uploadAssets <Badge type="tip" text="POST" /> <Badge type="warning" text="测试" />
:上传单个资源到后端，用于测试。

**端点URL:**
```url
/api/upload
```
**URL参数:**
- *无*

**请求头:**
- `Content-Type: application/json`
- `Authorization: Bearer <token>`

**请求体:**
```json
{
    "file": bytes[]
}
```

**成功响应:**
- **代码:** `200 OK`
- **内容:**
```json
{
    "message": "资源上传成功！",
    "asset-id": uuid()
}
```

**错误响应:**
- **代码:** `400 Bad Request` / `401 Unauthorized`/ `404 Not Found`
- **内容:**
```json
{
    "error": "错误信息:", string
}
```

## batchUploadAssets <Badge type="tip" text="POST" />
:批量上传资源到后端

**端点URL:**
```url
/api/batch-upload
```
**URL参数:**
- *无*

**请求头:**
- `Content-Type: application/json`
- `Authorization: Bearer <token>`

**请求体:**
```json
files: File[]      // 多文件，以form-data格式上传
```

**成功响应:**
- **代码:** `200 OK`
- **内容:**
```json
{
    "results": message,
    "total":  上传请求的资源总数,
    "successful": 成功上传的资源数量,
}
```

**错误响应:**
- **代码:** `400 Bad Request` / `401 Unauthorized`/ `404 Not Found`
- **内容:**
```json
{
    "error": "错误信息:", string
}
```