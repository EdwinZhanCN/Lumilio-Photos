# AssetsStore Anatomy

> Store composition with 5 slices: Entities, Views, UI, Filters, and Selection

```mermaid
classDiagram
    direction TB
    
    %% Main Store combines all slices
    class AssetsStore {
        <<Zustand Store>>
    }
    
    %% ========== ENTITIES SLICE ==========
    class EntitiesSlice {
        <<Slice>>
        %% State
        +Record~id, Asset~ assets
        +Record~id, EntityMeta~ meta
        
        %% Actions
        +setEntity(id, asset, meta)
        +updateEntity(id, updates, meta)
        +deleteEntity(id)
        +batchSetEntities(assets, meta)
    }
    
    %% ========== VIEWS SLICE ==========
    class ViewsSlice {
        <<Slice>>
        %% State
        +Record~key, ViewState~ views
        +string[] activeViewKeys
        
        %% Actions
        +createView(key, definition)
        +setViewLoading(key, loading)
        +setViewAssets(key, ids, hasMore, pageInfo)
        +appendViewAssets(key, ids, hasMore, pageInfo)
        +setViewError(key, error)
        +setViewLoadingMore(key, loading)
        +removeView(key)
        +removeAssetFromViews(assetId)
        +cleanupStaleViews(maxAge)
    }
    
    %% ========== UI SLICE ==========
    class UISlice {
        <<Slice>>
        %% State
        +TabType currentTab
        +GroupByType groupBy
        +string searchQuery
        +SearchMode searchMode
        +boolean isCarouselOpen
        +string activeAssetId
        
        %% Actions
        +setCurrentTab(tab)
        +setGroupBy(groupBy)
        +setSearchQuery(query)
        +setSearchMode(mode)
        +setCarouselOpen(isOpen)
        +setActiveAssetId(id)
        +hydrateUIFromURL(params)
    }
    
    %% ========== FILTERS SLICE ==========
    class FiltersSlice {
        <<Slice>>
        %% State
        +boolean enabled
        +boolean raw
        +number rating
        +boolean liked
        +FilenameFilter filename
        +DateRange date
        +string camera_make
        +string lens
        
        %% Actions
        +setFiltersEnabled(enabled)
        +setFilterRaw(raw)
        +setFilterRating(rating)
        +setFilterLiked(liked)
        +setFilterFilename(filename)
        +setFilterDate(date)
        +setFilterCameraMake(make)
        +setFilterLens(lens)
        +resetFilters()
        +batchUpdateFilters(updates)
    }
    
    %% ========== SELECTION SLICE ==========
    class SelectionSlice {
        <<Slice>>
        %% State
        +boolean enabled
        +Set~string~ selectedIds
        +SelectionMode selectionMode
        +string lastSelectedId
        
        %% Actions
        +toggleAssetSelection(id)
        +clearSelection()
        +setSelectionEnabled(enabled)
        +selectAsset(id)
        +deselectAsset(id)
        +selectAll(ids)
        +setSelectionMode(mode)
    }
    
    %% ========== SELECTORS (Hooks) ==========
    class Selectors {
        <<Consumer Hooks>>
        %% Selection
        +useSelectionEnabled()
        +useSelectedIds()
        +useSelectedCount()
        +useIsAssetSelected(id)
        +useSelectionMode()
        
        %% UI
        +useCurrentTab()
        +useGroupBy()
        +useSearchQuery()
        +useSearchMode()
        +useIsCarouselOpen()
        +useActiveAssetId()
        
        %% Filters
        +useFiltersEnabled()
        +useActiveFilterCount()
        +useFilterState()
        
        %% Entities
        +useAsset(id)
        +useAssetMeta(id)
        +useAllAssets()
        
        %% Views
        +useView(key)
        +useViewAssetIds(key)
        
        %% Action Hooks
        +useSelectionActions()
        +useUIActions()
        +useFilterActions()
    }
    
    %% ========== EXTERNAL DEPENDENCIES ==========
    class AssetService {
        <<API>>
        +listAssets(params)
        +getAssetById(id)
        +deleteAsset(id)
        +filterAssets(request)
        +searchAssets(request)
        +updateAssetMetadata(id, request)
        +updateAssetRating(id, rating)
        +updateAssetLike(id, liked)
    }
    
    %% Relationships
    AssetsStore *-- EntitiesSlice : composes
    AssetsStore *-- ViewsSlice : composes
    AssetsStore *-- UISlice : composes
    AssetsStore *-- FiltersSlice : composes
    AssetsStore *-- SelectionSlice : composes
    
    Selectors ..> AssetsStore : subscribes
    ViewsSlice ..> AssetService : fetches via
    EntitiesSlice ..> AssetService : mutations via
```

## Slice Responsibilities

| Slice | Purpose |
|-------|---------|
| **EntitiesSlice** | Normalized asset data cache (single source of truth for asset objects) |
| **ViewsSlice** | Manages paginated lists of asset IDs for different views/queries |
| **UISlice** | UI state like current tab, search query, carousel visibility |
| **FiltersSlice** | Advanced filter state (rating, date range, camera, lens, etc.) |
| **SelectionSlice** | Multi-select functionality for bulk operations |
