/**
 * # Home
 *
 * Home owns the authenticated `/` dashboard: featured media, library
 * statistics, and a bounded spacetime-map preview. It is a read-oriented
 * composition surface; editing, asset browsing, collection membership, upload
 * destinations, and preference persistence remain with their owning features.
 *
 * ## State
 *
 * {@link Home} keeps its `gallery`/`stats` mode in the `tab` URL parameter; the
 * default gallery mode omits the parameter. Repository selection comes from
 * {@link useBrowseScope}, so "all repositories" remains a valid scope. Home
 * never reads the working repository because it creates no media.
 *
 * {@link StatsCards} keeps only its selected heatmap year locally and resets it
 * when repository scope changes. Featured assets, statistics, map points, and
 * location clusters are independent TanStack Query server facts.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/"] --> HOME["Home"]
 *     HOME --> SCOPE["BrowseScopeSelect"]
 *     HOME --> GALLERY["featured gallery"]
 *     HOME --> STATS["statistics"]
 *     HOME --> MAP["spacetime map"]
 *     GALLERY --> ASSETS["AssetPreviewGrid"]
 *     MAP --> MAPENTRY["Assets map entry"]
 *     MAP --> ROUTEASSET["/assets/:assetId"]
 * ```
 *
 * {@link GalleryGrid} delegates finite asset presentation to Assets.
 * {@link SpacetimeMapCard} is lazy-loaded only when its card nears the viewport
 * and delegates map rendering to {@link PhotoMapView}. Selecting a map point
 * navigates to the owning asset route rather than opening an editor in Home.
 *
 * ## Data
 *
 * {@link useFeaturedPhotos} reads `/api/v1/assets/featured` with a small result
 * count and larger candidate window. {@link usePhotoStats} coordinates
 * focal-length, camera/lens, time-distribution, available-year, and daily
 * activity queries without merging them into client state.
 *
 * {@link useMapPhotoAssets} requests a bounded map-point preview after the map
 * becomes visible. {@link useLocationClusters} supplies the place count. Trips
 * may drain those paginated sources elsewhere because grouping requires a
 * complete dataset; Home deliberately does not.
 *
 * @module
 */
import type { useLocationClusters } from "../assets/map/useLocationClusters.ts";
import type { useMapPhotoAssets } from "../assets/map/useMapPhotoAssets.ts";
import type { useBrowseScope } from "../repositories/index.ts";
import type { useFeaturedPhotos } from "./api/useFeaturedPhotos.ts";
import type { usePhotoStats } from "./api/usePhotoStats.ts";
import type GalleryGrid from "./flows/overview/GalleryGrid.tsx";
import type Home from "./flows/overview/HomeFlow.tsx";
import type PhotoMapView from "./flows/overview/PhotoMapView.tsx";
import type SpacetimeMapCard from "./flows/overview/SpacetimeMapCard.tsx";
import type StatsCards from "./flows/overview/StatsCards.tsx";

export {};
