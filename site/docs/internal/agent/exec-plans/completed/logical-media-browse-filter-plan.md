# Destructive PR Plan: Logical Media Browse & Filtering

> - 路径：`site/docs/internal/agent/exec-plans/active/logical-media-browse-filter-plan.md`
> - 目标分支：`experimental/sqlite`
> - 状态：In progress
> - PR Title：`feat(browse)!: make media items the canonical browse and filter unit`
> - Merge strategy：单 PR 开发，最终 squash merge 为一个 breaking commit

## PR Identity

**Core promise**

图库、筛选、搜索和 stack 展示全部以 `media_item` 为用户级最小单位。`asset` 只代表物理文件，不再直接充当图库行。

---

# 1. Destructive Decisions

本 PR 不保留任何旧行为或兼容入口。

删除以下 contract：

```text
filter.raw
QueryAssetsParams.IsRaw
?raw=true
?raw=false
Agent filter_assets.raw
BrowseItem.type = "asset"
BrowseItem ID = asset:<asset_id>
total_assets
expanded = one physical asset per row
```

不提供：

* `raw=true → contains_raw` 映射
* 旧 URL 参数迁移
* 旧 Agent pin/ref plan 迁移
* 旧 SQLite catalog 数据迁移
* DTO deprecated alias
* 新旧 query 双轨
* feature flag
* compatibility adapter

旧实验 catalog 必须重建。旧 URL 中的 `raw` 参数不会被识别。旧客户端会编译或请求失败，这是预期行为。

---

# 2. Final Domain Model

SQLite schema 已经存在三层结构：

```text
assets
物理文件，例如 CR3、JPEG、MOV

media_items
用户认知中的逻辑媒体，例如 JPEG+RAW、Live Photo、编辑版本集合

asset_stacks
多个 media_item 的展示分组，例如 burst、manual
```

`media_item_assets.relation` 已包含：

```text
raw_original
jpeg_original
edited_version
live_photo_still
live_photo_video
alternative
```

`asset_stacks.stack_kind` 已限定为：

```text
burst
manual
```

这两个维度保持完全正交。

---

# 3. Final Filter Contract

## 3.1 API DTO

```go
type MediaComposition string

const (
    MediaCompositionContainsRAW MediaComposition = "contains_raw"
    MediaCompositionJPEGRAW     MediaComposition = "jpeg_raw"
    MediaCompositionRAWUnpaired MediaComposition = "raw_unpaired"
    MediaCompositionNoRAW       MediaComposition = "no_raw"
)

type StackMembership string

const (
    StackMembershipStacked   StackMembership = "stacked"
    StackMembershipUnstacked StackMembership = "unstacked"
)

type MediaItemFilterDTO struct {
    Composition *MediaComposition `json:"composition,omitempty"`
}

type StackFilterDTO struct {
    Membership *StackMembership `json:"membership,omitempty"`
    Kinds      []string         `json:"kinds,omitempty" enums:"burst,manual"`
}

type AssetFilterDTO struct {
    RepositoryID *string             `json:"repository_id,omitempty"`
    AlbumID      *int                `json:"album_id,omitempty"`
    Type         *string             `json:"type,omitempty"`
    Types        []string            `json:"types,omitempty"`
    OwnerID      *int32              `json:"owner_id,omitempty"`
    MediaItem    *MediaItemFilterDTO `json:"media_item,omitempty"`
    Stack        *StackFilterDTO     `json:"stack,omitempty"`
    Rating       *int                `json:"rating,omitempty"`
    Liked        *bool               `json:"liked,omitempty"`
    Filename     *FilenameFilterDTO  `json:"filename,omitempty"`
    Date         *DateRangeDTO       `json:"date,omitempty"`
    IsDeleted    *bool               `json:"is_deleted,omitempty"`
    CameraModel  *string             `json:"camera_model,omitempty"`
    Lens         *string             `json:"lens,omitempty"`
    Location     *LocationBBoxDTO    `json:"location,omitempty"`
    TagName      *string             `json:"tag_name,omitempty"`
    TagSource    *string             `json:"tag_source,omitempty"`
    TagNames     []string            `json:"tag_names,omitempty"`
    PersonID     *int32              `json:"person_id,omitempty"`
    FolderPath   *string             `json:"folder_path,omitempty"`
    FolderRecursive *bool            `json:"folder_recursive,omitempty"`
}
```

彻底删除 `RAW *bool`。

## 3.2 Composition Semantics

| 值              | SQL 语义                         |
| -------------- | ------------------------------ |
| `contains_raw` | `has_raw = 1`                  |
| `jpeg_raw`     | `has_raw = 1 AND has_jpeg = 1` |
| `raw_unpaired` | `has_raw = 1 AND has_jpeg = 0` |
| `no_raw`       | `has_raw = 0`                  |

`raw_unpaired` 可以同时包含 edited component，但不能包含 `jpeg_original`。

## 3.3 Stack Semantics

| 请求                     | SQL 语义                     |
| ---------------------- | -------------------------- |
| 未设置                    | 不限制 stack                  |
| `membership=stacked`   | `stack_id IS NOT NULL`     |
| `membership=unstacked` | `stack_id IS NULL`         |
| `kinds=["burst"]`      | `stack_kind IN ('burst')`  |
| `kinds=["manual"]`     | `stack_kind IN ('manual')` |

验证规则：

* `membership=unstacked` 与非空 `kinds` 同时提交时返回 400。
* 非空 `kinds` 自动表示 stacked，不要求重复提交 membership。
* 空数组标准化为未设置。
* 未知 composition、membership 或 kind 返回 400。

---

# 4. Destructive SQLite Baseline

## 4.1 Rewrite Baseline In Place

直接修改：

```text
server/migrations/000001_sqlite_baseline.up.sql
```

不增加旧 catalog 的升级 migration。

在 baseline 末尾设置：

```sql
PRAGMA user_version = 3;
```

应用启动时：

1. 检查 catalog 是否已经存在核心表。
2. 若是空数据库，正常执行 baseline。
3. 若核心表存在但 `PRAGMA user_version != 3`，拒绝启动。
4. 错误信息明确要求删除或重建 catalog。

示例：

```text
This catalog belongs to an incompatible experimental schema generation.
Delete the SQLite catalog and restart Lumilio Photos.
Media repositories and original files are not deleted.
```

## 4.2 New Browse Facts View

在 baseline 中增加权威 view：

```sql
CREATE VIEW media_item_browse_facts AS
SELECT
    mi.media_item_id,
    mi.owner_id,
    mi.repository_id,
    mi.media_kind,
    mi.primary_asset_id,

    COUNT(mia.asset_id) AS component_count,

    MAX(CASE WHEN mia.relation = 'raw_original' THEN 1 ELSE 0 END) AS has_raw,
    MAX(CASE WHEN mia.relation = 'jpeg_original' THEN 1 ELSE 0 END) AS has_jpeg,
    MAX(CASE WHEN mia.relation = 'edited_version' THEN 1 ELSE 0 END) AS has_edited,
    MAX(CASE WHEN mia.relation = 'live_photo_video' THEN 1 ELSE 0 END) AS has_live_motion,

    asm.stack_id,
    asm.position AS stack_position,
    s.stack_kind

FROM media_items mi
JOIN media_item_assets mia
  ON mia.media_item_id = mi.media_item_id
LEFT JOIN asset_stack_members asm
  ON asm.media_item_id = mi.media_item_id
LEFT JOIN asset_stacks s
  ON s.stack_id = asm.stack_id
GROUP BY
    mi.media_item_id,
    mi.owner_id,
    mi.repository_id,
    mi.media_kind,
    mi.primary_asset_id,
    asm.stack_id,
    asm.position,
    s.stack_kind;
```

此 view 是 composition 与 stack filter 的唯一派生来源。

## 4.3 Required Indexes

```sql
CREATE INDEX idx_media_item_assets_item_relation
ON media_item_assets(media_item_id, relation, asset_id);

CREATE INDEX idx_media_items_primary_asset
ON media_items(primary_asset_id);

CREATE INDEX idx_media_items_repository_owner
ON media_items(repository_id, owner_id, media_item_id);

CREATE INDEX idx_asset_stack_members_stack_position
ON asset_stack_members(stack_id, position, media_item_id);

CREATE INDEX idx_asset_stacks_kind
ON asset_stacks(stack_kind, stack_id);
```

移除已经不再服务 browse filter 的：

```text
idx_assets_is_raw_text_active
```

现有 baseline 只有单列 `media_item_assets(media_item_id)`，尚无 relation 复合索引。

---

# 5. Ingest Invariants

## 5.1 Never Insert Generic `original` for Known Photos

当前新 asset 创建 media item 时，component relation 被固定写为 `original`。

替换：

```sql
-- name: AttachOriginalAssetToMediaItem
...
'original'
```

为：

```sql
-- name: AttachAssetToMediaItem
INSERT INTO media_item_assets (
    asset_id,
    media_item_id,
    relation,
    position,
    created_at
) VALUES (
    sqlc.arg('asset_id'),
    sqlc.arg('media_item_id'),
    sqlc.arg('relation'),
    0,
    sqlc.arg('created_at')
);
```

新增统一分类函数：

```go
func InitialMediaRelation(
    validation *file.ValidationResult,
    filename string,
) repo.StackRelation
```

规则：

```text
RAW file      → raw_original
JPEG file     → jpeg_original
other photo   → original
video/audio   → original
```

该函数由 materializer 和 stack service 共用，禁止各自重新判断扩展名。

## 5.2 Metadata Reconciliation

Metadata task 完成后再次校验 component relation：

* metadata 确认是 RAW 时，更新为 `raw_original`
* JPEG metadata/MIME 时，更新为 `jpeg_original`
* 已经属于 `live_photo_*` 或 `edited_version` 时不覆盖
* 操作必须幂等

这样可以覆盖扩展名错误、MIME 错误和 scanner 边缘输入。

## 5.3 Primary Asset Rules

逻辑照片的 primary asset 按以下顺序选择：

1. `jpeg_original`
2. `live_photo_still`
3. `edited_version`
4. `raw_original`
5. position 最小的其他 component

任何 component 增删或 relation 变化后，都调用：

```go
NormalizeMediaItemPrimaryAsset(ctx, mediaItemID)
```

不再只在 structural merge 时临时挑 JPEG。

---

# 6. Stack Detection Lifecycle

## 6.1 Move Detection After Metadata Completion

当前 upload handler 在 ingest 尚未完成时独立 enqueue detection，而且只有请求显式提供 `repository_id` 时才执行。

删除 upload handler 中的 detection enqueue。

统一流程：

```text
materialize asset
→ metadata extraction succeeds
→ reconcile component relation
→ enqueue LivePhotoMatchArgs(asset_id)
→ enqueue DetectStacksArgs(resolved repository_id)
```

适用于：

* upload
* chunked upload
* repository scan
* cloud import
* retry

`DetectStacksArgs` 保持 repository 级唯一任务和幂等重跑。

## 6.2 Detection Order

每次 `DetectStacks` 执行：

1. 归一化现有 media item component relations
2. 合并 JPEG/RAW structural components
3. 合并 edited versions
4. 运行 Live Photo 后置一致性检查
5. 检测 exact EXIF burst
6. 检测保守的 timestamp/filename burst
7. 规范化受影响 presentation stacks
8. 规范化受影响 media item primary asset

## 6.3 Presentation Stack Invariants

新增：

```go
NormalizePresentationStack(ctx, stackID)
```

规则：

* 0 member：删除 stack
* 1 member：删除 membership，再删除 stack
* cover 缺失：选择 position 最小的 member
* position 重复或不连续：重排为 `0..n-1`
* stack owner/repository 必须与所有 member 一致

在以下操作后调用：

* structural media merge
* remove from stack
* delete media item
* delete asset
* move component
* manual stack creation
* burst extension

数据库中不得存在单成员 presentation stack。

---

# 7. Canonical Browse Query

## 7.1 Query Starts From `media_item_browse_facts`

当前 collapsed SQL 从 primary asset 开始，因此 JPEG primary 会导致 JPEG+RAW 无法通过 RAW 条件。

新 query skeleton：

```sql
WITH eligible_media AS (
    SELECT
        facts.*,
        primary_asset.upload_time,
        primary_asset.taken_time
    FROM media_item_browse_facts facts
    JOIN assets primary_asset
      ON primary_asset.asset_id = facts.primary_asset_id
    WHERE
        primary_asset.is_deleted = COALESCE(sqlc.narg('is_deleted'), false)

        AND composition predicate
        AND stack predicate

        AND existing primary-asset metadata predicates
)
```

## 7.2 Existing Filter Scope

以下 filter 继续作用于 primary asset：

```text
type
date
rating
liked
camera
lens
location
tag
album
person
repository
owner
deleted state
```

`filename` 改为匹配 media item 任意 component：

```sql
EXISTS (
    SELECT 1
    FROM media_item_assets mia_name
    JOIN assets component_name
      ON component_name.asset_id = mia_name.asset_id
    WHERE mia_name.media_item_id = facts.media_item_id
      AND filename predicate
)
```

Source/pin/ref asset ID scope同样匹配任意 component：

```sql
EXISTS (
    SELECT 1
    FROM media_item_assets mia_source
    WHERE mia_source.media_item_id = facts.media_item_id
      AND mia_source.asset_id IN (...)
)
```

这样 source 只包含 RAW component 时，仍能返回逻辑照片的 JPEG primary。

## 7.3 Rewrite All Query Variants

全部重写，不保留 `is_raw` 参数：

```text
server/internal/db/repo/queries/assets_01.sql
server/internal/db/repo/queries/assets_02.sql
server/internal/db/repo/queries/assets_03.sql
server/internal/db/repo/queries/assets_04.sql
server/internal/db/repo/queries/assets_05.sql
```

职责：

```text
assets_01
Agent/ref ordered logical media producer

assets_02
Expanded media-item browse

assets_03
Media-item and file counts

assets_04
Collapsed presentation browse

assets_05
Collapsed visible count
```

删除所有：

```sql
sqlc.narg('is_raw')
json_extract(a.specific_metadata, '$.is_raw')
```

## 7.4 Presentation Modes

保留名称 `stack_mode`，但重新定义：

```text
collapsed
一个 presentation stack 一行
未堆叠 media item 一行

expanded
每个匹配 media item 一行
stack 不折叠，但仍附带 stack metadata
```

expanded 不再返回 physical assets。当前实现直接调用 `QueryAssets` 并逐 asset 输出，必须删除。

---

# 8. New Browse Response Contract

## 8.1 Media Item DTO

```go
type MediaCompositionDTO struct {
    ComponentCount int  `json:"component_count"`
    HasRAW         bool `json:"has_raw"`
    HasJPEG        bool `json:"has_jpeg"`
    HasEdited      bool `json:"has_edited"`
    HasLiveMotion  bool `json:"has_live_motion"`
}

type BrowseMediaItemDTO struct {
    MediaItemID  string              `json:"media_item_id"`
    MediaKind    string              `json:"media_kind"`
    PrimaryAsset AssetDTO            `json:"primary_asset"`
    Composition  MediaCompositionDTO `json:"composition"`
    Stack        *StackPreviewDTO    `json:"stack,omitempty"`
}
```

## 8.2 Stack DTO

```go
type BrowseStackMemberDTO struct {
    MediaItemID   string `json:"media_item_id"`
    PrimaryAssetID string `json:"primary_asset_id"`
}

type BrowseStackDTO struct {
    StackID        string                   `json:"stack_id"`
    StackKind      string                   `json:"stack_kind"`
    Cover          BrowseMediaItemDTO       `json:"cover"`
    Members        []BrowseStackMemberDTO   `json:"members"`
    MatchedMembers []BrowseStackMemberDTO   `json:"matched_members"`
}
```

## 8.3 Browse Item

```go
type BrowseItemDTO struct {
    Type      string              `json:"type" enums:"media_item,stack"`
    ID        string              `json:"id"`
    MediaItem *BrowseMediaItemDTO `json:"media_item,omitempty"`
    Stack     *BrowseStackDTO     `json:"stack,omitempty"`
    BestTsMs  *int32              `json:"best_ts_ms,omitempty"`
}
```

ID：

```text
media:<media_item_id>
stack:<stack_id>
```

删除：

```text
type=asset
asset field
cover_asset_id
member_asset_ids
matched_member_ids
```

## 8.4 Counts

响应只返回：

```json
{
  "total_visible": 120,
  "total_media_items": 150,
  "total_files": 213
}
```

定义：

* `total_visible`：当前 presentation mode 下的图库行数
* `total_media_items`：命中的逻辑媒体数量
* `total_files`：命中 media items 的未删除 component 文件数量

彻底删除 `total_assets`。

---

# 9. Service Layer

修改 `QueryAssetsParams`：

```go
type QueryAssetsParams struct {
    // existing fields

    MediaComposition *MediaComposition
    StackMembership  *StackMembership
    StackKinds       []dbtypes.StackKind

    StackMode string
    Limit     int
    Offset    int
}
```

删除：

```go
IsRaw *bool
```

重构：

```text
QueryAssets
```

改为内部 physical-file helper，仅供明确的文件级任务使用，不再作为 browse expanded 实现。

新增：

```go
QueryMediaItems
QueryCollapsedBrowseItems
CountMediaItems
CountMediaItemFiles
```

`attachBrowseStackKinds` 删除，因为 stack kind 直接由主 SQL 返回。当前额外查询不再需要。

---

# 10. Search

## 10.1 Search Results Always Flat by Media Item

`/assets/search` 删除 `stack_mode` 参数。

搜索返回：

```text
一行一个 media item
附带所属 stack 的 preview metadata
不折叠 presentation stack
```

原因是 relevance ranking 不能被 stack collapse 重排。

## 10.2 Retriever Resolution

```text
retriever asset IDs
→ map each asset to media_item_id
→ apply composition and stack filters
→ dedupe by media_item_id
→ retain highest-ranked matching asset contribution
→ hydrate media item primary asset
→ preserve relevance order
```

对于视频：

* `best_ts_ms` 继续来自具体匹配 video asset
* media item primary 必须仍可打开该视频

## 10.3 Filename Search

filename search 与 semantic/OCR search 使用同一个 media-item hydration 和 filtering pipeline，不允许出现普通 browse 与 search 语义不同的情况。

---

# 11. Agent Filter

当前 Agent tool 仍提供 `raw *bool`，并描述为 “RAW photos only”。

彻底替换输入：

```go
type AssetFilterInput struct {
    DateFrom   string   `json:"date_from,omitempty"`
    DateTo     string   `json:"date_to,omitempty"`
    Type       string   `json:"type,omitempty"`
    Filename   string   `json:"filename,omitempty"`

    Composition   string   `json:"composition,omitempty"`
    StackMembership string `json:"stack_membership,omitempty"`
    StackKinds    []string `json:"stack_kinds,omitempty"`

    Rating     *int     `json:"rating,omitempty"`
    Liked      *bool    `json:"liked,omitempty"`
    Place      string   `json:"place,omitempty"`
    Camera     string   `json:"camera,omitempty"`
    Lens       string   `json:"lens,omitempty"`
    AlbumID    *int     `json:"album_id,omitempty"`
    TagNames   []string `json:"tag_names,omitempty"`
}
```

Agent ref snapshot：

* 每个 media item 只产生一个 ID
* snapshot 使用 primary asset ID
* 同一个 JPEG+RAW media item 不得出现两个 ID
* stack filter 先作用于 media item，再生成 snapshot
* summary 使用 `media items`，不再写 `assets`

示例：

```text
filter(composition=jpeg_raw, stack_kinds=burst) → 42 media items
```

旧 pins、refs 和 checkpoints 不迁移，因为 catalog 整体重建。

---

# 12. Frontend Filter State

## 12.1 New Type

```ts
type MediaComposition =
  | "contains_raw"
  | "jpeg_raw"
  | "raw_unpaired"
  | "no_raw";

type StackMembership = "stacked" | "unstacked";
type StackKind = "burst" | "manual";

type AssetUserFilter = {
  type?: "PHOTO" | "VIDEO";

  media_item?: {
    composition?: MediaComposition;
  };

  stack?: {
    membership?: StackMembership;
    kinds?: StackKind[];
  };

  rating?: number;
  liked?: boolean;
  filename?: FilenameFilter;
  date?: DateFilter;
  camera_model?: string;
  lens?: string;
  tag_names?: string[];
  location?: AssetLocationBBox;
};
```

删除所有：

```text
raw?: boolean
rawEnabled
rawMode
RawSection
```

## 12.2 Simplify Filter Draft

删除“整个 filter 启用开关”和每个 section 的 enabled boolean。

最终 draft 直接保存值：

```ts
type FilterDraft = {
  type?: "PHOTO" | "VIDEO";
  composition?: MediaComposition;
  stackMembership?: StackMembership;
  stackKinds: StackKind[];
  rating?: number;
  liked?: boolean;
  filename?: FilenameFilter;
  date?: DateFilter;
  location?: AssetLocationBBox;
  cameraModel?: string;
  lens?: string;
  tagNames: string[];
};
```

规则：

* 值存在即 active
* “全部”对应 `undefined`
* reset section 只清除该 section
* reset all 清空所有非 locked 条件
* locked constraint 使用只读状态和锁图标

当前双层 enable 状态整体删除。

## 12.3 UI Sections

### 媒体构成

```text
[全部] [含 RAW]
[JPEG+RAW] [RAW 未配对]
[不含 RAW]
```

### 分组

```text
[全部] [未分组] [所有堆叠]
[连拍] [手动堆叠]
```

选择“连拍”或“手动堆叠”时不需要再选择“所有堆叠”。

## 12.4 URL Contract

只支持：

```text
composition=jpeg_raw
stack_membership=stacked
stack_kind=burst
stack_kind=manual
```

删除：

```text
raw=true
raw=false
```

不解析、不迁移旧参数。

## 12.5 Active Chips

Header 显示：

```text
JPEG + RAW ×
连拍 ×
Canon EOS R5 ×
```

每个 chip 可独立清除。

## 12.6 Thumbnail Badges

Media item badges：

```text
RAW
JPEG+RAW
LIVE
EDITED
```

Presentation badges：

```text
burst icon + member count
manual stack icon + member count
```

部分 stack 命中显示：

```text
2 / 7
```

JPEG+RAW 禁止使用 stack icon。

## 12.7 Preserve Stack Kind

当前 DTO 转换会丢弃 `stack_kind`。新 `BrowseStackItem` 必须保留：

```ts
stackKind: "burst" | "manual";
```

并用于 badge、filter state 和 viewer context。

---

# 13. Event Integration

Event 保持 browse grouping 维度，不变成 stack kind 或 composition。

最终 pipeline：

```text
repository / album / event scope
→ media-item predicates
→ composition filter
→ stack filter
→ presentation collapse or expansion
→ pagination
→ event/date grouping
→ render
```

Event 中包含 JPEG+RAW burst 时：

* Event 负责决定分组边界
* burst 负责决定 presentation collapse
* composition 负责决定是否命中

三者互不覆盖。

本 PR 不重写 event detection algorithm，只调整它消费的 browse item identity，从 physical asset 改为 media item。

---

# 14. Tests

## 14.1 Real SQLite Integration Matrix

建立真实临时 SQLite catalog fixture：

1. standalone JPEG
2. standalone RAW
3. JPEG+RAW
4. JPEG+RAW+edited
5. RAW+edited，无 JPEG
6. Live Photo
7. 普通 burst
8. 每帧都是 JPEG+RAW 的 burst
9. manual stack of JPEG+RAW items
10. mixed manual stack，只有部分 item 含 RAW
11. source scope 只包含 RAW component ID
12. stack cover 不命中但其他 member 命中
13. structural merge 后 stack 只剩一个 member
14. 删除 JPEG 后由 `jpeg_raw` 变成 `raw_unpaired`
15. 删除 RAW 后由 `jpeg_raw` 变成 `no_raw`

每组验证：

```text
contains_raw
jpeg_raw
raw_unpaired
no_raw
stacked
unstacked
burst
manual
collapsed
expanded
list/count parity
pagination
matched members
cover fallback
```

## 14.2 Invariant Tests

验证数据库永远满足：

```text
每个 asset 只属于一个 media item
每个 media item 至少一个 component
每个 media item primary 指向自己的 component
每个 stack 至少两个 media items
每个 media item 最多属于一个 presentation stack
stack cover 必须属于 stack
position 连续且唯一
```

## 14.3 Handler Contract Tests

删除旧测试中的：

```text
StackModeExpanded_AssetBrowseItems
total_assets
type=asset
member_asset_ids
```

新增：

```text
type=media_item
media:<media_item_id>
total_media_items
total_files
composition validation
stack filter validation
search rejects stack_mode
```

## 14.4 Frontend Tests

覆盖：

* composition normalize
* stack normalize
* illegal unstacked + kinds
* URL round-trip
* old `raw` URL ignored
* locked composition
* locked stack kind
* active chip count
* individual chip clear
* stack kind preservation
* badge rendering
* partial match rendering
* reload state preservation

## 14.5 Agent Tests

覆盖：

* JPEG+RAW 只生成一个 snapshot ID
* `jpeg_raw + burst`
* `contains_raw + manual`
* invalid composition
* invalid stack kind
* no legacy `raw` schema
* summary 使用 media items

## 14.6 E2E

完整路径：

```text
导入 JPEG+RAW 连拍
→ metadata 完成
→ 自动合并为 media items
→ 自动识别 burst stack
→ 选择 JPEG+RAW
→ 选择连拍
→ 图库只显示一个 burst stack
→ badge 显示 JPEG+RAW 和 burst count
→ 打开 stack
→ 每个 item 可切换 JPEG/RAW component
→ 切换 expanded
→ 显示各 media item，不显示重复物理文件
→ 清除连拍
→ standalone JPEG+RAW 出现
```

---

# 15. Performance Gates

在合并前建立至少三档 benchmark catalog：

```text
10k media items
100k media items
1M media items
```

场景：

* 无 filter collapsed
* `contains_raw`
* `jpeg_raw`
* `stack_kind=burst`
* `jpeg_raw + burst`
* repository + date + composition
* source asset IDs only containing secondary components
* expanded pagination
* collapsed pagination

最低验收：

* list 与 count query 均使用新增 relation/stack indexes
* 禁止对全部 `assets.specific_metadata` 做 RAW JSON scan
* 100k catalog 第一页查询不出现明显秒级停顿
* count 和 rows 不允许运行不同 predicate
* `EXPLAIN QUERY PLAN` 作为测试 snapshot 或 benchmark artifact 保存

---

# 16. Primary File Map

## Backend

```text
server/migrations/000001_sqlite_baseline.up.sql
server/internal/sourcing/materializer.go
server/internal/processors/metadata_task.go

server/internal/db/repo/queries/assets_00.sql
server/internal/db/repo/queries/assets_01.sql
server/internal/db/repo/queries/assets_02.sql
server/internal/db/repo/queries/assets_03.sql
server/internal/db/repo/queries/assets_04.sql
server/internal/db/repo/queries/assets_05.sql
server/internal/db/repo/queries/stacks.sql
server/internal/db/repo/queries/stack_kinds.sql

server/internal/service/asset_service.go
server/internal/service/asset_browse_service.go
server/internal/service/asset_search_fused.go
server/internal/service/stack_service.go

server/internal/api/dto/asset_dto.go
server/internal/api/dto/stack_dto.go
server/internal/api/handler/asset_handler.go

server/internal/agent/tools/asset_filter.go
server/internal/agent/facets/facets.go
server/internal/agent/pins/pins.go
```

## Frontend

```text
web/src/features/assets/model/filter.ts
web/src/features/assets/model/browseRouteState.ts
web/src/features/assets/model/browseItems.ts
web/src/features/assets/types.ts

web/src/features/assets/api/assetViewModel.ts
web/src/features/assets/api/useAssetsList.ts
web/src/features/assets/api/useAssetsSearch.ts

web/src/features/assets/flows/browse/filtering/types.ts
web/src/features/assets/flows/browse/filtering/filterState.ts
web/src/features/assets/flows/browse/filtering/FilterTool.tsx
web/src/features/assets/flows/browse/filtering/sections/ChoiceSections.tsx

web/src/locales/en/translation.json
web/src/locales/zh/translation.json
web/src/lib/http-commons/schema.d.ts
```

## Generated Outputs

```text
server/internal/db/repo/*.sql.go
server/docs/swagger.yaml
server/docs/swagger.json
server/docs/docs.go
web/src/lib/http-commons/schema.d.ts
```

---

# 17. Single-PR Execution Order

即使最终只提交一个 squash commit，开发顺序必须固定：

1. 重写 baseline、schema generation 和 indexes
2. 修 ingest component relation
3. 加 media/stack invariant normalization
4. 移动 detection 调度到 metadata completion
5. 重写 SQL query 与 count
6. 重写 service 和 DTO
7. 重写 search hydration
8. 重写 Agent filter
9. regenerate sqlc、OpenAPI、TypeScript
10. 重写前端 browse item identity
11. 重写 filter state、URL 和 UI
12. 接入 badges 与 partial matches
13. 更新 event grouping consumer
14. 完成 SQLite integration matrix
15. 完成 frontend、Agent、E2E
16. 跑 architecture guard 和 performance benchmarks
17. squash merge

任何中间状态都不应合入目标分支。

---

# 18. Definition of Done

PR 只有在以下全部成立时才可合并：

* `raw` 字段在 Go、SQL、OpenAPI、TypeScript、URL 和 Agent schema 中完全消失
* 普通图库不再以 physical asset 为行单位
* standalone RAW 可被 `contains_raw` 和 `raw_unpaired` 找到
* JPEG+RAW 只显示一次
* JPEG+RAW 可被 `contains_raw` 和 `jpeg_raw` 找到
* JPEG+RAW 不被误认为 presentation stack
* burst/manual 可独立筛选
* composition 与 stack kind 可组合
* collapsed 与 expanded 命中同一组 media items
* expanded 不显示 JPEG/RAW 重复行
* source 只包含 secondary component ID 时仍能找到 media item
* search 每个 media item 只返回一次
* Agent snapshot 每个 media item 只保存一次
* stack 中部分 item 命中时 cover、matched members 和 focus 正确
* 数据库中没有 0 或 1 member stack
* `total_visible`、`total_media_items`、`total_files` 精确
* list/count predicate 完全一致
* old catalog 被明确拒绝，不尝试升级
* old URL、old pin、old ref 不做迁移
* 所有生成文件已更新
* SQLite、server、web、desktop、E2E 和 architecture guard 全部通过

---

# 19. Explicitly Out of Scope

本 PR 不做：

* PostgreSQL 数据迁移
* 旧 SQLite catalog 升级
* 旧 URL 或 API 兼容
* 旧 Agent checkpoint/pin/ref 兼容
* 独立 physical-file manager UI
* event detection algorithm 重写
* 新 stack kind
* HDR、panorama 等新自动分组算法
* media-item 级 rating/tag/album 数据模型迁移

这些功能不得作为“顺便支持”重新引入兼容分支。
