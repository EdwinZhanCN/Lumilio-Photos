# Async Data Flow Diagrams

## 1. Filter Assets Flow

> User applies filters → fetch matching assets → update views

```mermaid
sequenceDiagram
    participant UI as FilterPanel (Consumer)
    participant Store as AssetsStore
    participant Hook as useAssetView Hook
    participant API as $api
    
    Note over UI, API: User toggles filter (e.g., rating ≥ 4)
    
    UI->>Store: setFilterRating(4)
    activate Store
    Store->>Store: filters.rating = 4
    Store-->>UI: (re-render: filter badge shows)
    deactivate Store
    
    Note over Hook: Hook detects filter change
    Hook->>Store: setViewLoading(viewKey, true)
    Store-->>UI: (re-render: Loading=true)
    
    Hook->>API: filterAssets({ rating: 4, ... })
    activate API
    API-->>Hook: { assets: [...], total, hasMore }
    deactivate API
    
    Hook->>Store: batchSetEntities(assets)
    activate Store
    Store->>Store: entities.assets[id] = asset (for each)
    deactivate Store
    
    Hook->>Store: setViewAssets(viewKey, assetIds, hasMore, pageInfo)
    activate Store
    Store->>Store: views[viewKey].assetIds = [...]
    Store->>Store: views[viewKey].isLoading = false
    Store-->>UI: (re-render: Grid shows filtered results)
    deactivate Store
```

---

## 2. Search Assets Flow

> Filename or semantic search with debounced input

```mermaid
sequenceDiagram
    participant UI as SearchInput (Consumer)
    participant Store as AssetsStore
    participant Hook as useAssetView Hook
    participant API as $api
    
    Note over UI, API: User types "sunset" in search
    
    UI->>Store: setSearchQuery("sunset")
    activate Store
    Store->>Store: ui.searchQuery = "sunset"
    Store-->>UI: (input reflects typed value)
    deactivate Store
    
    Note over Hook: Debounce (300ms) then trigger search
    
    Hook->>Store: setViewLoading(viewKey, true)
    Store-->>UI: (Loading spinner)
    
    alt Semantic Search Mode
        Hook->>API: searchAssets({ query: "sunset", mode: "semantic" })
    else Filename Search Mode
        Hook->>API: searchAssets({ query: "sunset", mode: "filename" })
    end
    
    activate API
    API-->>Hook: { assets: [...], total }
    deactivate API
    
    Hook->>Store: batchSetEntities(assets)
    Hook->>Store: setViewAssets(viewKey, assetIds, hasMore, pageInfo)
    Store-->>UI: (Grid shows search results)
```

---

## 3. Infinite Scroll / Load More Flow

> Append new assets when scrolling to bottom

```mermaid
sequenceDiagram
    participant UI as AssetGrid (Consumer)
    participant Store as AssetsStore
    participant Hook as useAssetView Hook
    participant API as $api
    
    Note over UI, API: User scrolls near bottom
    
    UI->>Hook: onLoadMore()
    Hook->>Store: Check views[viewKey].hasMore
    
    alt hasMore = false
        Note over Hook: No action needed
    else hasMore = true
        Hook->>Store: setViewLoadingMore(viewKey, true)
        Store-->>UI: (Show loading indicator at bottom)
        
        Hook->>API: listAssets({ cursor: nextCursor, ... })
        activate API
        API-->>Hook: { assets: [...], nextCursor, hasMore }
        deactivate API
        
        Hook->>Store: batchSetEntities(assets)
        Hook->>Store: appendViewAssets(viewKey, newIds, hasMore, pageInfo)
        activate Store
        Store->>Store: views[viewKey].assetIds.push(...newIds)
        Store->>Store: views[viewKey].isLoadingMore = false
        Store-->>UI: (Grid appends new items)
        deactivate Store
    end
```

---

## 4. Asset Update Flow (Rating/Like)

> Optimistic update pattern for instant feedback

```mermaid
sequenceDiagram
    participant UI as AssetCard (Consumer)
    participant Store as AssetsStore
    participant API as $api
    
    Note over UI, API: User clicks star to rate 5
    
    UI->>Store: updateEntity(assetId, { rating: 5 })
    activate Store
    Store->>Store: entities.assets[id].rating = 5 (optimistic)
    Store-->>UI: (Star fills immediately)
    deactivate Store
    
    UI->>API: updateAssetRating(id, 5)
    activate API
    
    alt Success
        API-->>UI: { success: true }
        Note over UI: No action needed (already updated)
    else Error
        API-->>UI: { error: "..." }
        UI->>Store: updateEntity(assetId, { rating: oldRating })
        Store-->>UI: (Rollback: star reverts)
    end
    deactivate API
```

---

## 5. Selection & Bulk Delete Flow

> Multi-select then bulk operation

```mermaid
sequenceDiagram
    participant UI as SelectionToolbar
    participant Store as AssetsStore
    participant API as $api
    
    Note over UI, API: User selects multiple assets
    
    UI->>Store: setSelectionEnabled(true)
    Store-->>UI: (Selection mode activated)
    
    loop For each tap on asset
        UI->>Store: toggleAssetSelection(assetId)
        Store->>Store: selection.selectedIds.add/remove(id)
        Store-->>UI: (Checkbox updates)
    end
    
    Note over UI: User clicks "Delete Selected"
    
    UI->>Store: Get selectedIds from store
    
    loop For each selectedId
        UI->>API: deleteAsset(id)
        API-->>UI: { success: true }
        UI->>Store: deleteEntity(id)
        UI->>Store: removeAssetFromViews(id)
    end
    
    UI->>Store: clearSelection()
    UI->>Store: setSelectionEnabled(false)
    Store-->>UI: (Selection mode deactivated, assets removed from grid)
```

---

## Key Patterns Summary

| Pattern | When to Use |
|---------|-------------|
| **Optimistic Update** | Rating, like, quick metadata edits |
| **Loading → Fetch → Update** | Initial load, filter changes, search |
| **Append** | Infinite scroll / pagination |
| **Batch + Cleanup** | Bulk delete, bulk tag operations |
