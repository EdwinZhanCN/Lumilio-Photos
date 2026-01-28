# API Modules Documentation

This directory contains API service modules and references to related helpers for the Lumilio Photos application. All API access uses TypeScript types generated from the OpenAPI schema.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Available API Modules](#available-api-modules)
- [Type Safety](#type-safety)
- [Usage Examples](#usage-examples)

## Overview

Most API modules follow a consistent pattern:

1. **Type Aliases**: Import and alias types from the generated OpenAPI schema (`schema.d.ts`)
2. **API Wrapper**: Expose either a service object or React Query hooks
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

## Available API Modules

### 1. Upload Helpers & Hooks

Handles file uploads (single, batch, and chunked).

**Types (from `@/lib/upload/types`):**
- `UploadResponse` - Single upload result
- `BatchUploadResponse` - Batch upload results
- `BatchUploadResult` - Individual file result in batch

**Transport Helpers (from `@/lib/upload/uploadTransport`):**
```typescript
uploadFile(file: File, hash: string, options?: UploadOptions)
batchUploadFiles(files: BatchUploadFile[], repositoryId?: string, options?: BatchUploadOptions)
uploadFileInChunks(file: File, sessionId: string, hash: string, chunkSize?: number, repositoryId?: string, onProgress?: (progress: number) => void, options?: ChunkedUploadOptions)
generateSessionId(): string
shouldUseChunks(file: File, threshold?: number): boolean
```

**React Query Hooks (from `@/features/upload/hooks`):**
- `useUploadFileMutation`
- `useBatchUploadMutation`
- `useChunkedUploadMutation`
- `useUploadConfig`
- `useUploadProgress`

### 2. Assets Types & URLs

Asset operations now use React Query via `$api` from `queryClient.ts`. Asset types live in `lib/assets/types.ts`, and URL helpers live in `lib/assets/assetUrls.ts`.

**Types (from `@/lib/assets/types`):**
- `Asset` - Asset data transfer object
- `AssetListResponse` - Paginated asset list
- `AssetFilter` - Filter criteria
- `SearchAssetsRequest` - Search parameters
- `FilterAssetsRequest` - Filter parameters
- And many more...

**URL Helpers (from `@/lib/assets/assetUrls`):**
```typescript
assetUrls.getOriginalFileUrl(id: string): string
assetUrls.getThumbnailUrl(id: string, size?: "small" | "medium" | "large"): string
assetUrls.getWebVideoUrl(id: string): string
assetUrls.getWebAudioUrl(id: string): string
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
type Asset = Schemas["dto.AssetDTO"];

// Access path parameter types
type ListAssetsParams =
  NonNullable<Paths["/api/v1/assets"]["get"]["parameters"]["query"]>;
```

### Request/Response Types

Every `$api` call is fully typed:

```typescript
// Typed response
const optionsQuery = $api.useQuery("get", "/api/v1/assets/filter-options", {});

// Access typed data
const options = optionsQuery.data?.data;
```

## Usage Examples

### Example 1: Asset URL Helpers

```typescript
import { assetUrls } from "@/lib/assets/assetUrls";

const thumbnail = assetUrls.getThumbnailUrl("asset-id", "medium");
```

### Example 2: Upload Files with Progress

```typescript
import { useBatchUploadMutation } from "@/features/upload/hooks/useUploadMutations";
import { generateSessionId } from "@/lib/upload/uploadTransport";
import type { BatchUploadFile, BatchUploadResult } from "@/lib/upload/types";

function useUploadPhotos() {
  const batchUpload = useBatchUploadMutation();

  return async (files: File[]) => {
    const uploadFiles: BatchUploadFile[] = files.map((file) => ({
      file,
      sessionId: generateSessionId(),
    }));

    const response = await batchUpload.mutateAsync({
      files: uploadFiles,
      options: {
        onUploadProgress: (progressEvent) => {
          const percent = Math.round(
            (progressEvent.loaded * 100) / (progressEvent.total || 1),
          );
          console.log(`Upload progress: ${percent}%`);
        },
      },
    });

    const results: BatchUploadResult[] = response.data?.results || [];
    const successful = results.filter((r) => r.success);
    console.log(
      `${successful.length}/${results.length} files uploaded successfully`,
    );

    return results;
  };
}
```

### Example 3: Search Assets Semantically

```typescript
import { $api } from "@/lib/http-commons/queryClient";
import type { SearchAssetsRequest } from "@/lib/assets/types";

const request: SearchAssetsRequest = {
  query: "sunset",
  search_type: "semantic",
  limit: 20,
  filter: { type: "PHOTO", liked: true }
};

const searchQuery = $api.useQuery("post", "/api/v1/assets/search", {
  body: request,
});

const assets = searchQuery.data?.data?.assets || [];
```

### Example 4: Create Album and Add Photos

```typescript
import { albumService, type CreateAlbumRequest } from "@/services";

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
import { $api } from "@/lib/http-commons/queryClient";
import type { FilterAssetsRequest, AssetFilter } from "@/lib/assets/types";

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

const filterQuery = $api.useQuery("post", "/api/v1/assets/filter", {
  body: request,
});

const assets = filterQuery.data?.data?.assets || [];
```

## Best Practices

1. **Always import types**: Use `type` imports for better tree-shaking
   ```typescript
   import type { Asset } from "@/lib/assets/types";
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
