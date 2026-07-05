/**
 * # Home
 *
 * The Home feature owns the authenticated `/` landing dashboard. It is a
 * read-oriented composition page for featured photos, library statistics, and
 * the spacetime map. It does not own asset browsing, upload targets, collection
 * membership, or settings persistence; it consumes shared hooks and routes the
 * user to the feature that owns the selected object.
 *
 * ## State
 *
 * {@link Home} stores only view state from the URL. The default `gallery` view
 * omits the `tab` query parameter; the statistics view is addressed as
 * `?tab=stats`. The page header includes {@link BrowseScopeSelect}, and the
 * selected browse scope is read through {@link useBrowseScope}.
 *
 * Browse scope is the only repository preference Home observes. When the user
 * chooses a repository, the scoped id is passed to featured-photo, statistics,
 * map-point, and location-cluster hooks. Home does not use the working
 * repository because it never creates new assets.
 *
 * {@link StatsCards} owns only local presentation state for its selected
 * heatmap year. Changing repository scope resets the selected year so the
 * default can be recalculated from the scoped available-years response.
 *
 * ## Data
 *
 * {@link useFeaturedPhotos} reads `/api/v1/assets/featured` with a small count
 * and a larger candidate window. {@link GalleryGrid} renders those assets
 * through the shared square gallery grouping helpers and uses skeleton cards
 * when no featured assets have loaded.
 *
 * {@link usePhotoStats} coordinates focal-length, camera/lens, time-of-day,
 * available-years, and daily-activity stats. The hook currently fetches these
 * endpoints together and lets {@link StatsCards} transform the raw response
 * into percentages and heatmap values.
 *
 * {@link useMapPhotoAssets} reads paginated map points from
 * `/api/v1/assets/map-points` and automatically drains additional pages while
 * more map data is available. {@link useLocationClusters} reads paginated
 * location clusters for the map badge. {@link SpacetimeMapCard} delegates map
 * rendering to {@link PhotoMapView}; clicking a point navigates to the owning
 * asset route instead of opening an editor inside Home.
 *
 * ## Composition
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/"] --> HOME["Home"]
 *     HOME --> SCOPE["BrowseScopeSelect / useBrowseScope"]
 *     HOME --> TABS["gallery or stats tab"]
 *     TABS --> GALLERY["GalleryGrid"]
 *     TABS --> STATS["StatsCards"]
 *     HOME --> MAP["SpacetimeMapCard"]
 *     GALLERY --> FEATURED["useFeaturedPhotos"]
 *     STATS --> PHOTO_STATS["usePhotoStats"]
 *     MAP --> MAP_POINTS["useMapPhotoAssets"]
 *     MAP --> CLUSTERS["useLocationClusters"]
 *     MAP --> PHOTO_MAP["PhotoMapView"]
 * ```
 *
 * Home composes already-owned surfaces: gallery rendering comes from Assets,
 * repository scope comes from Settings, heatmap rendering comes from shared
 * components, and map presentation comes from the shared map component.
 *
 * ## Decisions
 *
 * Home is overview-first. It favors compact summaries and deep links over
 * editing controls, because the authoritative asset and collection workflows
 * live elsewhere.
 *
 * Repository scope is browse scope. "All repositories" is a valid Home scope;
 * upload's concrete working repository is intentionally not used here.
 *
 * Map data is paginated but drained opportunistically. The page can render a
 * partial map while more points load, and the subtitle reports loaded/total
 * counts rather than blocking the whole dashboard.
 *
 * @module
 */
import type BrowseScopeSelect from "@/components/BrowseScopeSelect";
import type PhotoMapView from "@/components/PhotoMapView";
import type Home from "./routes/Home.tsx";
import type GalleryGrid from "./components/GalleryGrid.tsx";
import type SpacetimeMapCard from "./components/SpacetimeMapCard.tsx";
import type StatsCards from "./components/StatsCards.tsx";
import type { useBrowseScope } from "@/features/settings";
import type { useFeaturedPhotos } from "./hooks/useFeaturedPhotos.ts";
import type { useLocationClusters } from "./hooks/useLocationClusters.ts";
import type { useMapPhotoAssets } from "./hooks/useMapPhotoAssets.ts";
import type { usePhotoStats } from "./hooks/usePhotoStats.ts";

export {};
