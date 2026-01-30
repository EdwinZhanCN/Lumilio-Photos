# AssetsStore Anatomy

> Store composition with 4 slices: UI, Filters, Selection, and Entities (Deprecated)

```mermaid
classDiagram
    direction TB
    
    %% Main Store combines all slices
    class AssetsStore {
        <<Zustand Store>>
    }
    
    %% ========== ENTITIES SLICE (DEPRECATED) ==========
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
        
        %% Entities (Deprecated)
        +useAsset(id)
        +useAssetMeta(id)
        +useAllAssets()
        
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
    AssetsStore *-- UISlice : composes
    AssetsStore *-- FiltersSlice : composes
    AssetsStore *-- SelectionSlice : composes
    
    Selectors ..> AssetsStore : subscribes
    EntitiesSlice ..> AssetService : mutations via
```

## Slice Responsibilities

| Slice | Purpose |
|-------|---------|
| **EntitiesSlice** | (Deprecated) Normalized asset data cache. Now assets are managed by React Query. |
| **UISlice** | UI state like current tab, search query, carousel visibility |
| **FiltersSlice** | Advanced filter state (rating, date range, camera, lens, etc.) |
| **SelectionSlice** | Multi-select functionality for bulk operations |
