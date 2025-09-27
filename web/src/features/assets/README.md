## Asset

### State Management

**(Provider、Router、Service、Routes、Hooks)**
```mermaid
flowchart TB
  subgraph Provider[AssetsProvider (Context + Reducers + Effects)]
    direction TB
    S1[AssetsState\n{entities, views, ui, filters, selection}]
    D1([dispatch])
    E1[[effects:\n- init from URL + settings\n- persist to localStorage(filters, selection)\n- sync URL<->UI\n- sync carousel by route param\n- cleanup stale views]]
    S1 <-.- D1
  end

  subgraph Router[react-router]
    L[location.pathname]
    Q[searchParams (?groupBy, ?q)]
    P[params.assetId]
  end

  subgraph External
    Settings[SettingsContext]
    Storage[(localStorage)]
    API[assetsService\n(list/search/update/delete...)]
    Geo[geoService]
    Workers[WorkerProvider]
  end

  subgraph Routes[Feature Routes]
    A1[Assets.tsx\n<AssetsProvider>…]
    Pht[Photos.tsx]
    Vid[Videos.tsx]
    Aud[Audios.tsx]
    Tabs[AssetTabs]
  end

  subgraph UI[Components + Hooks]
    Hd[AssetsPageHeader\n(GroupBy, SearchBar, FilterTool, SelectionToggle)]
    Masonry[PhotosMasonry -> PhotosThumbnail]
    Carousel[FullScreenCarousel]
    Info[FullScreenBasicInfo]
    hook1[useAssetsContext]
    hook2[useCurrentTabAssets/useAssetsView]
    hook3[useAssetActions]
    hook4[useSelection]
  end

  Settings --> Provider
  Workers --> Routes
  Storage <--> Provider
  Router --> Provider
  Provider --> Routes
  A1 --> Pht & Vid & Aud & Tabs

  Hd --> hook1
  Masonry --> hook1
  Carousel --> hook1
  Info --> hook1
  Hd --> hook2
  Pht --> hook2

  hook2 --> API
  API --> Provider
  hook3 --> API
  Info --> Geo

  %% URL <-> UI sync
  Hd -- change groupBy/search --> D1
  Provider -- write URL --> Router
  Router -- read URL --> Provider

  %% Navigation helpers
  Hd & Masonry & Carousel -- open/close/switch --> Provider

  %% Entities usage
  Provider -. exposes state .-> UI
  UI -. renders assets from selectors .-> Provider
```

**Root Reducer**
```mermaid
classDiagram
  class AssetsState {
    +entities: EntitiesState
    +views: ViewsState
    +ui: UIState
    +filters: FiltersState
    +selection: SelectionState
  }

  class EntitiesState {
    +assets: Record<string, Asset>
    +meta: Record<string, EntityMeta>
  }

  class ViewsState {
    +views: Record<string, ViewState>
    +activeViewKeys: string[]
  }

  class UIState {
    +currentTab: "photos"|"videos"|"audios"
    +groupBy: "date"|"type"|"album"|"flat"
    +searchQuery: string
    +searchMode: "filename"|"semantic"
    +isCarouselOpen: boolean
    +activeAssetId?: string
  }

  class FiltersState {
    +enabled: boolean
    +raw?: boolean
    +rating?: number
    +liked?: boolean
    +filename?: { mode, value }
    +date?: { from, to }
    +camera_make?: string
    +lens?: string
  }

  class SelectionState {
    +enabled: boolean
    +selectedIds: Set<string>
    +lastSelectedId?: string
    +selectionMode: "single"|"multiple"
  }

  class Reducers {
    +assetsReducer()
    +entitiesReducer()
    +viewsReducer()
    +uiReducer()
    +filtersReducer()
    +selectionReducer()
  }

  AssetsState --> EntitiesState
  AssetsState --> ViewsState
  AssetsState --> UIState
  AssetsState --> FiltersState
  AssetsState --> SelectionState

  Reducers ..> EntitiesState
  Reducers ..> ViewsState
  Reducers ..> UIState
  Reducers ..> FiltersState
  Reducers ..> SelectionState
```

**Data Fetch/View Cache**

```mermaid
sequenceDiagram
  actor U as UI(Photos/Videos/Audios)
  participant H as useCurrentTabAssets / useAssetsView
  participant Ctx as AssetsContext(state,dispatch)
  participant V as views.reducer
  participant S as assetsService
  participant E as entities.reducer

  U->>H: 构造 ViewDefinition(types, groupBy, search, inheritGlobalFilter)
  H->>Ctx: dispatch(CREATE_VIEW)
  Ctx->>V: 创建/激活 ViewState(viewKey)

  alt autoFetch 初次加载
    H->>Ctx: dispatch(SET_VIEW_LOADING=true)
    H->>S: listAssets()/searchAssets(params from def + global filters)
    S-->>H: { data: { assets[], offset? } }
    H->>Ctx: dispatch(BATCH_SET_ENTITIES(assets))
    Ctx->>E: 归一化存入 entities/meta
    H->>Ctx: dispatch(SET_VIEW_ASSETS({assetIds, hasMore, pageInfo}))
    Ctx->>V: 保存 assetIds, hasMore, lastFetchAt
    U-->>U: 通过 selectors 渲染 assets 或 groups
  end

  opt 分页加载
    U->>H: fetchMore()
    H->>Ctx: dispatch(SET_VIEW_LOADING_MORE=true)
    H->>S: list/search(next page)
    S-->>H: 返回下一页 assets
    H->>Ctx: dispatch(BATCH_SET_ENTITIES)
    H->>Ctx: dispatch(APPEND_VIEW_ASSETS)
    Ctx->>V: 去重后追加IDs, 更新pageInfo/hasMore
  end

  note over H, V: 视图 key 通过 generateViewKey(definition) 稳定缓存
```

**URL <-> UI State bijection, Carousel**

```
sequenceDiagram
  actor User
  participant UI as Component(Masonry/Thumbnail/Header)
  participant Nav as useAssetsNavigation(navigate)
  participant Router as react-router
  participant Provider as AssetsProvider(Effects)
  participant UI2 as UI(Carosuel)

  User->>UI: 点击缩略图
  UI->>Nav: openCarousel(assetId)
  Nav->>Router: navigate(/assets/:tab/:assetId?groupBy=&q=)
  Router-->>Provider: params.assetId 变化
  Provider->>Provider: dispatch(SET_CAROUSEL_OPEN,true)\n dispatch(SET_ACTIVE_ASSET_ID, id)
  Provider-->>UI2: context.ui 更新(isCarouselOpen=true, activeAssetId=id)
  UI2-->>User: 渲染 FullScreenCarousel

  User->>UI: 切换 GroupBy/输入 Search
  UI->>Provider: dispatch(SET_GROUP_BY / SET_SEARCH_QUERY)
  Provider->>Router: effect 写回 URL (?groupBy, ?q)\n(默认值则删除参数)
  Router-->>Provider: effect 读取 URL -> HYDRATE_UI_FROM_URL

  User->>UI2: 关闭轮播
  UI2->>Nav: closeCarousel()
  Nav->>Router: navigate(/assets/:tab?groupBy=&q=)
  Router-->>Provider: params.assetId 为空
  Provider->>Provider: dispatch(SET_CAROUSEL_OPEN,false)\n activeAssetId=undefined
```

**Selection and Batch Operations**

```mermaid
flowchart LR
  subgraph Selection_UI[选择交互]
    Hd[AssetsPageHeader: Selection Toggle]
    Th[PhotosThumbnail/Keyboard]
  end

  subgraph Hooks[Hooks]
    USel[useSelection]
    UOps[useBulkAssetOperations]
    UAct[useAssetActions]
  end

  subgraph Reducers[Reducers]
    SelR[selection.reducer]
    EntR[entities.reducer]
    ViewR[views.reducer]
  end

  Hd --> USel
  Th --> USel
  USel --> SelR

  UOps --> UAct
  UAct -- Optimistic UPDATE_ENTITY --> EntR
  UAct -. API .-> API[(assetsService)]
  API -- OK --> UAct
  UAct --> EntR:::confirm
  API -- Error --> UAct
  UAct --> EntR:::revert

  classDef confirm fill:#d3f9d8,stroke:#2b8a3e,color:#2b8a3e
  classDef revert fill:#ffe3e3,stroke:#c92a2a,color:#c92a2a

```

**Basic Info Panel**

```mermaid
sequenceDiagram
  participant P as Photos.tsx(局部: updatedAssets Map)
  participant FS as FullScreenCarousel
  participant Info as FullScreenBasicInfo
  participant Act as useAssetActions
  participant S as assetsService
  participant Ctx as AssetsContext
  participant Ent as entities.reducer
  participant View as views.reducer

  Note over Info: 评分/描述使用 React useOptimistic 本地乐观 UI
  Info->>S: updateAssetRating/Description
  S-->>Info: 200 OK
  Info->>FS: onAssetUpdate(updatedAsset)
  FS->>P: onAssetUpdate(updatedAsset)
  P->>P: updatedAssets.set(id, updated)\n渲染时合并 flatAssets

  Note over FS,Act: 删除使用全局 action，非本地乐观
  FS->>Act: deleteAsset(assetId)
  Act->>S: DELETE /assets/:id
  S-->>Act: 200 OK
  Act->>Ctx: dispatch(REMOVE_ASSET_FROM_VIEWS)
  Ctx->>View: 从各视图移除该ID
  Act->>Ctx: dispatch(DELETE_ENTITY)
  Ctx->>Ent: 从 entities/meta 删除
  FS->>FS: onClose() 关闭轮播

```
