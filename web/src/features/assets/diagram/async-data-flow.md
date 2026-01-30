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
    Hook->>API: filterAssets({ rating: 4, ... })
    activate API
    API-->>Hook: { assets: [...], total, hasMore }
    deactivate API
    
    Hook-->>UI: (re-render: Grid shows filtered results)
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
    
    alt Semantic Search Mode
        Hook->>API: searchAssets({ query: "sunset", mode: "semantic" })
    else Filename Search Mode
        Hook->>API: searchAssets({ query: "sunset", mode: "filename" })
    end
    
    activate API
    API-->>Hook: { assets: [...], total }
    deactivate API
    
    Hook-->>UI: (Grid shows search results)
```

---

## 3. Infinite Scroll / Load More Flow

> Append new assets when scrolling to bottom

```mermaid
sequenceDiagram
    participant UI as AssetGrid (Consumer)
    participant Hook as useAssetView Hook
    participant API as $api
    
    Note over UI, API: User scrolls near bottom
    
    UI->>Hook: onLoadMore()
    Hook->>Hook: Check hasMore
    
    alt hasMore = false
        Note over Hook: No action needed
    else hasMore = true
        Hook->>API: listAssets({ cursor: nextCursor, ... })
        activate API
        API-->>Hook: { assets: [...], nextCursor, hasMore }
        deactivate API
        
        Hook-->>UI: (Grid appends new items)
    end
```

---

## 4. Asset Update Flow (Rating/Like)

> Optimistic update pattern for instant feedback

```mermaid
sequenceDiagram
    participant UI as AssetCard (Consumer)
    participant QueryCache as ReactQuery
    participant API as $api
    
    Note over UI, API: User clicks star to rate 5
    
    UI->>QueryCache: updateAssetInQueries(assetId, { rating: 5 })
    activate QueryCache
    QueryCache-->>UI: (Star fills immediately)
    deactivate QueryCache
    
    UI->>API: updateAssetRating(id, 5)
    activate API
    
    alt Success
        API-->>UI: { success: true }
        Note over UI: No action needed (already updated)
    else Error
        API-->>UI: { error: "..." }
        UI->>QueryCache: updateAssetInQueries(assetId, { rating: oldRating })
        QueryCache-->>UI: (Rollback: star reverts)
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
    participant QueryCache as ReactQuery
    
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
        UI->>QueryCache: markAssetDeletedInQueries(id)
        UI->>API: deleteAsset(id)
        API-->>UI: { success: true }
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
