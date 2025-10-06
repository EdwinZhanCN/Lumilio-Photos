# Services Documentation

This directory contains all API service modules for the Lumilio Photos application. All services have been refactored to use TypeScript types generated from the OpenAPI schema.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Available Services](#available-services)
- [Type Safety](#type-safety)
- [Usage Examples](#usage-examples)
- [Migration Guide](#migration-guide)

## Overview

All service modules follow a consistent pattern:

1. **Type Aliases**: Import and alias types from the generated OpenAPI schema (`schema.d.ts`)
2. **Service Object**: Export a service object with typed methods
3. **Full Type Safety**: All requests and responses are fully typed

### Key Features

- ✅ **100% Type Safe**: All API calls use generated TypeScript types
- ✅ **Consistent API**: All services follow the same structure
- ✅ **Auto-completion**: Full IDE autocomplete support
- ✅ **Documentation**: JSDoc comments on all methods
- ✅ **Error Handling**: Proper typing for error responses

## Architecture

### Type Generation

Types are automatically generated from the OpenAPI specification using `openapi-typescript`:

```
Backend OpenAPI Spec → openapi-typescript → schema.d.ts → Service Modules
```

```shell
npx openapi-typescript ./public/swagger.yaml -o ./src/lib/http-commons/schema.d.ts
```

### Standard Response Wrapper

All API responses follow the standard wrapper format:

```typescript
export type ApiResult<T = unknown> = Omit<components["schemas"]["api.Result"], "data"> & {
  data?: T;
};
```

This provides:
- `code: number` - Business status code (0 for success)
- `message: string` - User-readable message
- `data?: T` - Typed response data
- `error?: string` - Debug error message (when applicable)

## Available Services

### 1. Upload Service (`uploadService.ts`)

Handles file uploads (single and batch).

**Types:**
- `UploadResponse` - Single upload result
- `BatchUploadResponse` - Batch upload results
- `BatchUploadResult` - Individual file result in batch

**Methods:**
```typescript
uploadService.uploadFile(file: File, hash: string, config?: AxiosRequestConfig)
uploadService.batchUploadFiles(files: { file: File; hash: string }[], config?: AxiosRequestConfig)
```

### 2. Assets Service (`assetsService.ts`)

Manages asset operations (photos, videos, audio, documents).

**Types:**
- `Asset` - Asset data transfer object
- `AssetListResponse` - Paginated asset list
- `AssetFilter` - Filter criteria
- `SearchAssetsRequest` - Search parameters
- `FilterAssetsRequest` - Filter parameters
- And many more...

**Key Methods:**
```typescript
assetService.listAssets(params: ListAssetsParams, config?: AxiosRequestConfig)
assetService.getAssetById(id: string, params?: GetAssetByIdParams, config?: AxiosRequestConfig)
assetService.deleteAsset(id: string, config?: AxiosRequestConfig)
assetService.updateAssetMetadata(id: string, request: UpdateAssetRequest, config?: AxiosRequestConfig)
assetService.filterAssets(request: FilterAssetsRequest, config?: AxiosRequestConfig)
assetService.searchAssets(request: SearchAssetsRequest, config?: AxiosRequestConfig)
assetService.getFilterOptions(config?: AxiosRequestConfig)
assetService.updateAssetRating(id: string, rating: number, config?: AxiosRequestConfig)
assetService.updateAssetLike(id: string, liked: boolean, config?: AxiosRequestConfig)
assetService.updateAssetDescription(id: string, description: string, config?: AxiosRequestConfig)
```

**URL Helpers:**
```typescript
assetService.getOriginalFileUrl(id: string): string
assetService.getThumbnailUrl(id: string, size?: "small" | "medium" | "large"): string
assetService.getWebVideoUrl(id: string): string
assetService.getWebAudioUrl(id: string): string
```

### 3. Album Service (`albumService.ts`)

Manages photo albums and their contents.

**Types:**
- `Album` - Album data object
- `ListAlbumsResponse` - Paginated album list
- `CreateAlbumRequest` - Album creation data
- `UpdateAlbumRequest` - Album update data

**Methods:**
```typescript
albumService.listAlbums(params?: ListAlbumsParams, config?: AxiosRequestConfig)
albumService.getAlbumById(id: number, config?: AxiosRequestConfig)
albumService.createAlbum(request: CreateAlbumRequest, config?: AxiosRequestConfig)
albumService.updateAlbum(id: number, request: UpdateAlbumRequest, config?: AxiosRequestConfig)
albumService.deleteAlbum(id: number, config?: AxiosRequestConfig)
albumService.getAlbumAssets(id: number, config?: AxiosRequestConfig)
albumService.addAssetToAlbum(albumId: number, assetId: string, request?: AddAssetToAlbumRequest, config?: AxiosRequestConfig)
albumService.removeAssetFromAlbum(albumId: number, assetId: string, config?: AxiosRequestConfig)
albumService.updateAssetPosition(albumId: number, assetId: string, request: UpdateAssetPositionRequest, config?: AxiosRequestConfig)
```

### 4. Auth Service (`authService.ts`)

Handles authentication and user management.

**Types:**
- `User` - User data object
- `AuthResponse` - Authentication response with tokens
- `LoginRequest` - Login credentials
- `RegisterRequest` - Registration data
- `RefreshTokenRequest` - Token refresh request

**Methods:**
```typescript
authService.login(request: LoginRequest, config?: AxiosRequestConfig)
authService.register(request: RegisterRequest, config?: AxiosRequestConfig)
authService.refreshToken(request: RefreshTokenRequest, config?: AxiosRequestConfig)
authService.logout(request: RefreshTokenRequest, config?: AxiosRequestConfig)
authService.getCurrentUser(config?: AxiosRequestConfig)
```

### 5. Health Service (`healthService.ts`)

Server health checking and monitoring.

**Functions:**
```typescript
checkHealth<T>(): Promise<HealthCheckResult<T>>
isServerOnline(): Promise<boolean>
pollHealth<T>(intervalSeconds: number, onUpdate: (result: HealthCheckResult<T>) => void): () => void
```

### 6. Geo Service (`geoService.ts`)

Reverse geocoding for GPS coordinates.

**Methods:**
```typescript
geoService.reverseGeocode(latitude: number, longitude: number, region?: string, language?: string): Promise<string>
```

### 7. Justified Layout Service (`justifiedLayoutService.ts`)

Photo grid layout calculation using WASM.

## Type Safety

### Using Generated Types

All types come from the generated schema:

```typescript
import type { components, paths } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];
type Paths = paths;

// Access schema types
type Asset = Schemas["handler.AssetDTO"];

// Access path parameter types
type ListAssetsParams = NonNullable<Paths["/assets"]["get"]["parameters"]["query"]>;
```

### Request/Response Types

Every service method is fully typed:

```typescript
// Typed request
const request: SearchAssetsRequest = {
  query: "sunset",
  search_type: "semantic",
  limit: 20,
  filter: {
    type: "PHOTO",
    rating: 5
  }
};

// Typed response
const response: AxiosResponse<ApiResult<AssetListResponse>> =
  await assetService.searchAssets(request);

// Access typed data
const assets: Asset[] = response.data.data?.assets || [];
```

## Usage Examples

### Example 1: List Assets with Filters

```typescript
import { assetService, type ListAssetsParams } from "@/services";

async function loadPhotos() {
  const params: ListAssetsParams = {
    type: "PHOTO",
    limit: 50,
    offset: 0,
    sort_order: "desc"
  };

  try {
    const response = await assetService.listAssets(params);

    if (response.data.code === 0) {
      const assets = response.data.data?.assets || [];
      console.log(`Loaded ${assets.length} photos`);
      return assets;
    }
  } catch (error) {
    console.error("Failed to load photos:", error);
  }
}
```

### Example 2: Upload Files with Progress

```typescript
import { uploadService, type BatchUploadResult } from "@/services";

async function uploadPhotos(files: File[]) {
  // Compute BLAKE3 hashes for files (implementation not shown)
  const filesWithHashes = await Promise.all(
    files.map(async (file) => ({
      file,
      hash: await computeBlake3Hash(file)
    }))
  );

  try {
    const response = await uploadService.batchUploadFiles(filesWithHashes, {
      onUploadProgress: (progressEvent) => {
        const percent = Math.round((progressEvent.loaded * 100) / (progressEvent.total || 1));
        console.log(`Upload progress: ${percent}%`);
      }
    });

    const results: BatchUploadResult[] = response.data.data?.results || [];
    const successful = results.filter(r => r.success);
    console.log(`${successful.length}/${results.length} files uploaded successfully`);

    return results;
  } catch (error) {
    console.error("Batch upload failed:", error);
  }
}
```

### Example 3: Search Assets Semantically

```typescript
import { assetService, type SearchAssetsRequest } from "@/services";

async function searchImages(query: string) {
  const request: SearchAssetsRequest = {
    query,
    search_type: "semantic",
    limit: 20,
    filter: {
      type: "PHOTO",
      liked: true  // Only search liked photos
    }
  };

  try {
    const response = await assetService.searchAssets(request);
    return response.data.data?.assets || [];
  } catch (error) {
    console.error("Search failed:", error);
    return [];
  }
}
```

### Example 4: Create Album and Add Photos

```typescript
import { albumService, assetService, type CreateAlbumRequest } from "@/services";

async function createVacationAlbum(assetIds: string[]) {
  // Create album
  const createRequest: CreateAlbumRequest = {
    album_name: "Summer Vacation 2024",
    description: "Beach trip photos",
    cover_asset_id: assetIds[0]
  };

  try {
    const albumResponse = await albumService.createAlbum(createRequest);
    const album = albumResponse.data.data;

    if (!album?.album_id) {
      throw new Error("Failed to create album");
    }

    // Add photos to album
    for (const assetId of assetIds) {
      await albumService.addAssetToAlbum(album.album_id, assetId);
    }

    console.log(`Created album with ${assetIds.length} photos`);
    return album;
  } catch (error) {
    console.error("Failed to create album:", error);
  }
}
```

### Example 5: User Authentication Flow

```typescript
import { authService, type LoginRequest, type AuthResponse } from "@/services";

async function loginUser(username: string, password: string) {
  const request: LoginRequest = {
    username,
    password
  };

  try {
    const response = await authService.login(request);
    const authData: AuthResponse | undefined = response.data.data;

    if (authData?.token) {
      // Store tokens
      localStorage.setItem("accessToken", authData.token);
      localStorage.setItem("refreshToken", authData.refreshToken || "");

      // Get user info
      const user = authData.user;
      console.log(`Logged in as ${user?.username}`);

      return authData;
    }
  } catch (error) {
    console.error("Login failed:", error);
    throw error;
  }
}
```

### Example 6: Filter Assets with Advanced Criteria

```typescript
import { assetService, type FilterAssetsRequest, type AssetFilter } from "@/services";

async function findRawPhotos() {
  const filter: AssetFilter = {
    type: "PHOTO",
    raw: true,
    rating: 4,
    camera_make: "Canon",
    date: {
      from: "2024-01-01T00:00:00Z",
      to: "2024-12-31T23:59:59Z"
    }
  };

  const request: FilterAssetsRequest = {
    filter,
    limit: 100,
    offset: 0
  };

  try {
    const response = await assetService.filterAssets(request);
    return response.data.data?.assets || [];
  } catch (error) {
    console.error("Filter failed:", error);
    return [];
  }
}
```

## Migration Guide

### Before (Old Code)

```typescript
// Custom interfaces
interface Asset {
  id: string;
  type: string;
  // ... manual type definitions
}

// Untyped API calls
const response = await api.get("/api/v1/assets");
const assets = response.data.data.assets; // No type safety
```

### After (Refactored Code)

```typescript
// Generated types
import { assetService, type Asset, type ListAssetsParams } from "@/services";

// Fully typed API calls
const params: ListAssetsParams = { type: "PHOTO", limit: 20 };
const response = await assetService.listAssets(params);
const assets: Asset[] = response.data.data?.assets || [];
```

### Key Changes

1. **Import from services**: Instead of calling `api` directly, use service methods
2. **Use generated types**: Replace custom interfaces with schema types
3. **Type parameters**: All query/path parameters are now typed
4. **Response typing**: Response shapes are fully typed with `ApiResult<T>`

### Breaking Changes

- `uploadService.ApiResult` is now properly typed from schema
- `assetService` methods now require typed request objects
- Some method signatures have changed to accept request objects instead of individual parameters

## Best Practices

1. **Always import types**: Use `type` imports for better tree-shaking
   ```typescript
   import { assetService, type Asset } from "@/services";
   ```

2. **Handle errors properly**: Check `response.data.code` for business logic errors
   ```typescript
   if (response.data.code !== 0) {
     console.error(response.data.message);
   }
   ```

3. **Use optional chaining**: Response data may be undefined
   ```typescript
   const assets = response.data.data?.assets || [];
   ```

4. **Leverage autocomplete**: Let TypeScript guide you with available fields
   ```typescript
   const filter: AssetFilter = {
     // TypeScript will show all available filter options
   };
   ```

5. **Keep services updated**: Regenerate schema types when backend API changes
   ```bash
   npm run generate-types
   ```

## Regenerating Types

When the backend API changes, regenerate the schema:

```bash
# Generate new schema from OpenAPI spec
npx openapi-typescript http://localhost:8080/swagger/doc.json -o src/lib/http-commons/schema.d.ts
```

The service modules will automatically pick up the new types.

## Contributing

When adding new service methods:

1. Use types from `schema.d.ts`
2. Follow the existing naming patterns
3. Add JSDoc comments
4. Include usage examples in this README
5. Run TypeScript checks: `npm run type-check`

## Questions?

For questions or issues with the services, please:
- Check the OpenAPI documentation at `/swagger/index.html`
- Review the generated `schema.d.ts` file
- Consult this README for usage examples
