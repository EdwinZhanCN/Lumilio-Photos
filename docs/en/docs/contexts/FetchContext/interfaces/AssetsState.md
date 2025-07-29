[lumilio-web](../../../modules.md) / [contexts/FetchContext](../index.md) / AssetsState

# Interface: AssetsState

Defined in: [contexts/FetchContext.tsx:71](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/0cb9b6c9a2e1869ca5ea4411f957d39edc719928/web/src/contexts/FetchContext.tsx#L71)

**Assets State Interface**

Defines the complete state structure for asset browsing operations.
This state is read-only and optimized for performance.

 AssetsState

## Since

1.0.0

## Example

```tsx
function AssetCounter() {
  const { assets, isLoading, hasMore } = useAssetsContext();

  return (
    <div>
      <p>Loaded {assets.length} assets</p>
      {isLoading && <p>Loading...</p>}
      {!hasMore && <p>All assets loaded</p>}
    </div>
  );
}
```

## Properties

### assets

> **assets**: `Asset`[]

Defined in: [contexts/FetchContext.tsx:76](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/0cb9b6c9a2e1869ca5ea4411f957d39edc719928/web/src/contexts/FetchContext.tsx#L76)

Array of currently loaded assets.
This list grows as more pages are fetched via infinite scrolling.

***

### error

> **error**: `null` \| `string`

Defined in: [contexts/FetchContext.tsx:86](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/0cb9b6c9a2e1869ca5ea4411f957d39edc719928/web/src/contexts/FetchContext.tsx#L86)

***

### filters

> **filters**: `ListAssetsParams`

Defined in: [contexts/FetchContext.tsx:83](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/0cb9b6c9a2e1869ca5ea4411f957d39edc719928/web/src/contexts/FetchContext.tsx#L83)

Current filter and search parameters applied to the asset list.

#### See

ListAssetsParams for available filter options

***

### hasMore

> **hasMore**: `boolean`

Defined in: [contexts/FetchContext.tsx:87](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/0cb9b6c9a2e1869ca5ea4411f957d39edc719928/web/src/contexts/FetchContext.tsx#L87)

***

### isLoading

> **isLoading**: `boolean`

Defined in: [contexts/FetchContext.tsx:84](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/0cb9b6c9a2e1869ca5ea4411f957d39edc719928/web/src/contexts/FetchContext.tsx#L84)

***

### isLoadingNextPage

> **isLoadingNextPage**: `boolean`

Defined in: [contexts/FetchContext.tsx:85](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/0cb9b6c9a2e1869ca5ea4411f957d39edc719928/web/src/contexts/FetchContext.tsx#L85)
