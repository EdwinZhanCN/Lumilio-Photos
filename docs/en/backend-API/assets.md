# Assets APIs
## uploadAssets <Badge type="tip" text="POST" /> <Badge type="warning" text="TEST" />
> Upload single asset to backend, testing usage

### Endpoint URL
```url
/api/upload
```

### Parameters
#### URL Parameters
None

#### Request Headers
- `Content-Type: application/json`
- `Authorization: Bearer <token>`

#### Request Body
```json
{
    "file": bytes[]
}
```

### Responses
#### Success Response
**Code:** `200 OK`
```json
{
    "message": "Asset Upload Success!",
    "asset-id": uuid()
}
```

#### Error Response
**Code:** `400 Bad Request` / `401 Unauthorized` / `404 Not Found`
```json
{
    "error": "Error message:", string
}
```

## batchUploadAssets <Badge type="tip" text="POST" />
> Upload multiple assets to backend simultaneously

### Endpoint URL
```url
/api/batch-upload
```

### Parameters
#### URL Parameters
None

#### Request Headers
- `Content-Type: multipart/form-data`
- `Authorization: Bearer <token>`

#### Request Body
```typescript
{
    files: File[]  // Multiple files in form-data format
}
```

### Responses
#### Success Response
**Code:** `200 OK`
```json
{
    "results": "string",     // Upload result message
    "total": number,         // Total number of assets requested
    "successful": number     // Number of successfully uploaded assets
}
```

#### Error Response
**Code:** `400 Bad Request` / `401 Unauthorized` / `404 Not Found`
```json
{
    "error": "string"        // Error message description
}
```

