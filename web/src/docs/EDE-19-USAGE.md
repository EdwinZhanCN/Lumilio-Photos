# EDE-19 Complete State Management For Photos Page - Usage Guide

## Overview

This document provides usage instructions for the completed state management implementation for the Photos page (EDE-19).

## Implementation Summary

### ✅ Completed Features

1. **Photos Page State Management** - Complete URL-synchronized state management
2. **Carousel Integration** - Fully functional full-screen carousel with navigation
3. **Asset Grouping** - Dynamic grouping by date, type, and album
4. **View Modes** - Masonry and Grid view support
5. **Search & Filtering** - Real-time search with debouncing
6. **Loading States** - Skeleton screens and proper loading indicators
7. **Error Handling** - Comprehensive error boundaries and fallbacks

### 🚧 Placeholder Features (Backend API Pending)

1. **Advanced Filters** - Date range, file size, camera/EXIF filtering
2. **EXIF Data Integration** - Two-implementation approach ready

## Usage Instructions

### Basic Navigation

```tsx
// The Photos page now supports URL-based navigation:
// /photos                          - Default view
// /photos?asset=123&carousel=true  - Open specific asset in carousel
// /photos?groupBy=type&sort=asc    - Custom grouping and sorting
// /photos?q=vacation               - Search query
// /photos?view=grid                - Grid view mode
```

### State Management Hook

```tsx
import { usePhotosPageState } from '@/hooks/usePhotosPageState';

// In your component:
const {
  // State
  selectedAssetId,
  isCarouselOpen,
  groupBy,
  sortOrder,
  viewMode,
  searchQuery,
  
  // Actions
  openCarousel,
  closeCarousel,
  setGroupBy,
  setSortOrder,
  setViewMode,
  setSearchQuery,
  updateCarouselIndex,
  navigateToAsset,
} = usePhotosPageState();
```

### Asset Grouping Utilities

```tsx
import { groupAssets, getFlatAssetsFromGrouped, findAssetIndex } from '@/utils/assetGrouping';

// Group assets
const groupedAssets = groupAssets(assets, 'date', 'desc');

// Get flat array for carousel
const flatAssets = getFlatAssetsFromGrouped(groupedAssets);

// Find asset position
const index = findAssetIndex(flatAssets, assetId);
```

### Component Integration

#### PhotosToolBar
```tsx
<PhotosToolBar
  groupBy={groupBy}
  sortOrder={sortOrder}
  viewMode={viewMode}
  searchQuery={searchQuery}
  onGroupByChange={setGroupBy}
  onSortOrderChange={setSortOrder}
  onViewModeChange={setViewMode}
  onSearchQueryChange={setSearchQuery}
  onShowExifData={(assetId) => {
    // TODO: Implement EXIF modal
    console.log('Show EXIF for:', assetId);
  }}
/>
```

#### PhotosMasonry
```tsx
<PhotosMasonry
  groupedPhotos={groupedPhotos}
  openCarousel={openCarousel}
  viewMode={viewMode}
  isLoading={isLoading}
  selectedAssetId={selectedAssetId}
/>
```

#### FullScreenCarousel
```tsx
{isCarouselOpen && selectedAssetId && (
  <FullScreenCarousel
    photos={flatAssets}
    initialSlide={currentIndex}
    onClose={closeCarousel}
    onNavigate={handleCarouselNavigate}
  />
)}
```

## EXIF Data Implementation Guide

### Two Implementation Approaches

#### 1. Backend Metadata (Quick Access)
```tsx
// Access existing metadata from API response
const exifFromBackend = asset.specificMetadata;

// Usage in component:
const cameraInfo = asset.specificMetadata?.camera;
const exposureInfo = asset.specificMetadata?.exposure;
```

#### 2. Client-Side Extraction (Full Data)
```tsx
import { useExtractExifdata } from '@/hooks/util-hooks/useExtractExifdata';

// In your component:
const { extractExifData, isExtracting, exifData } = useExtractExifdata({
  workerClientRef: yourWorkerRef
});

// Extract full EXIF data
await extractExifData([file]);

// Access comprehensive EXIF data
const fullExifData = exifData[0];
```

### When to Use Which Implementation

- **Backend Metadata**: Use for quick display, filtering, sorting
- **Client Extraction**: Use for detailed EXIF analysis, editing workflows

## Backend API Integration Points

### Pending API Endpoints

1. **Advanced Filtering**
```typescript
// When backend APIs are ready, update these in useAssetsContext:
interface ListAssetsParams {
  dateRange?: { start: string; end: string };
  fileSize?: { min: number; max: number };
  camera?: string;
  exifFilters?: Record<string, any>;
}
```

2. **EXIF Search**
```typescript
// Implement when backend supports EXIF-based search
const searchByExif = async (query: {
  camera?: string;
  lens?: string;
  iso?: number[];
  aperture?: number[];
}) => {
  // Backend call to search by EXIF criteria
};
```

## Performance Considerations

### Optimization Features

1. **Debounced Search** - 300ms delay to prevent excessive API calls
2. **URL State Persistence** - Maintains state across page refreshes
3. **Infinite Scroll** - Efficient loading of large asset collections
4. **Skeleton Loading** - Improves perceived performance
5. **Image Lazy Loading** - Only loads visible thumbnails

### Memory Management

- Thumbnail URLs are properly cleaned up
- Worker instances are terminated on unmount
- Event listeners are removed automatically

## Testing

### Manual Testing Checklist

- [ ] URL navigation works (direct links, back/forward buttons)
- [ ] Carousel navigation (keyboard, mouse, touch)
- [ ] Search functionality (real-time, debounced)
- [ ] View mode switching (masonry ↔ grid)
- [ ] Asset grouping (date, type, album)
- [ ] Sort order (newest/oldest first)
- [ ] Loading states (initial, infinite scroll)
- [ ] Error handling (network errors, empty states)
- [ ] Mobile responsiveness

### URL Test Cases

```bash
# Test these URLs manually:
/photos
/photos?groupBy=date&sort=desc
/photos?view=grid
/photos?q=test
/photos?asset=123&carousel=true&index=5
```

## Known Limitations

1. **Advanced Filters**: UI is placeholder until backend APIs are ready
2. **EXIF Filtering**: Client-side extraction is resource-intensive for large datasets
3. **Performance**: Large asset collections (>1000 items) may need virtualization

## Next Steps

1. **Backend Integration**: Implement advanced filtering APIs
2. **EXIF Modal**: Create dedicated EXIF data viewer component
3. **Performance**: Add virtualization for large collections
4. **Accessibility**: Enhance keyboard navigation and screen reader support
5. **Mobile**: Optimize touch gestures for carousel navigation

## Troubleshooting

### Common Issues

1. **Carousel not opening**: Check if `selectedAssetId` exists and `flatAssets` is populated
2. **URL not updating**: Ensure `useSearchParams` is properly imported from react-router-dom
3. **Infinite scroll not working**: Verify `useInView` hook setup and `hasNextPage` state
4. **Search not working**: Check debounce timing and context search integration

### Debug Tools

```tsx
// Add to Photos.tsx for debugging:
console.log('Photos Debug:', {
  groupedPhotos: Object.keys(groupedPhotos),
  flatAssetsCount: flatAssets.length,
  selectedAssetId,
  isCarouselOpen,
  currentParams: Object.fromEntries(searchParams.entries())
});
```
