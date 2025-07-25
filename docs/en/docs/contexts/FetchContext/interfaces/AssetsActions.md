[lumilio-web](../../../modules.md) / [contexts/FetchContext](../index.md) / AssetsActions

# Interface: AssetsActions

Defined in: [contexts/FetchContext.tsx:100](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/contexts/FetchContext.tsx#L100)

**Assets Actions Interface**

Defines all available actions for manipulating asset state.
These functions are stable and won't cause re-renders for components that only use them.

 AssetsActions

## Since

1.0.0

## Properties

### applyFilter()

> **applyFilter**: (`key`, `value`) => `void`

Defined in: [contexts/FetchContext.tsx:169](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/contexts/FetchContext.tsx#L169)

**Apply Filter Function**

Higher-level function to apply a new filter and refetch the asset list from the start.
Automatically resets pagination when filters change.

#### Parameters

##### key

keyof `ListAssetsParams`

The filter parameter to update

##### value

`any`

The new value for the filter parameter

#### Returns

`void`

#### Example

```tsx
// Filter by asset type
applyFilter('type', 'PHOTO');

// Filter by date range
applyFilter('dateRange', { start: '2024-01-01', end: '2024-01-31' });
```

***

### fetchAssets()

> **fetchAssets**: (`params`) => `Promise`\<`void`\>

Defined in: [contexts/FetchContext.tsx:121](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/contexts/FetchContext.tsx#L121)

**Fetch Assets Function**

Fetches the first page of assets based on new parameters, replacing the current list.
This is typically used when filters change or initial load occurs.

#### Parameters

##### params

`ListAssetsParams`

Filter and pagination parameters

#### Returns

`Promise`\<`void`\>

#### Example

```tsx
const { fetchAssets } = useAssetsContext();

// Fetch photos uploaded in the last week
await fetchAssets({
  type: 'PHOTO',
  dateRange: { start: '2024-01-01', end: '2024-01-07' },
  limit: 20
});
```

***

### fetchNextPage()

> **fetchNextPage**: () => `Promise`\<`void`\>

Defined in: [contexts/FetchContext.tsx:149](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/contexts/FetchContext.tsx#L149)

**Fetch Next Page Function**

Fetches the next page of assets and appends them to the current list.
Used for infinite scrolling implementations.

#### Returns

`Promise`\<`void`\>

#### Example

```tsx
function InfiniteScroll() {
  const { fetchNextPage, hasMore, isLoadingNextPage } = useAssetsContext();

  useEffect(() => {
    const handleScroll = () => {
      if (window.innerHeight + window.scrollY >= document.body.offsetHeight - 1000) {
        if (hasMore && !isLoadingNextPage) {
          fetchNextPage();
        }
      }
    };

    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, [fetchNextPage, hasMore, isLoadingNextPage]);
}
```

***

### resetFilters()

> **resetFilters**: () => `void`

Defined in: [contexts/FetchContext.tsx:221](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/contexts/FetchContext.tsx#L221)

**Reset Filters Function**

Resets all filters to their default state and refetches the complete asset list.
Useful for "Clear All" functionality.

#### Returns

`void`

#### Example

```tsx
<button onClick={resetFilters}>
  Clear All Filters
</button>
```

***

### setSearchQuery()

> **setSearchQuery**: (`query`) => `void`

Defined in: [contexts/FetchContext.tsx:206](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/contexts/FetchContext.tsx#L206)

**Set Search Query Function**

Higher-level function to apply a new search query and refetch the asset list.
Typically searches across filenames, descriptions, and tags.

#### Parameters

##### query

`string`

The search query string

#### Returns

`void`

#### Example

```tsx
function SearchBar() {
  const { setSearchQuery } = useAssetsContext();
  const [query, setQuery] = useState('');

  const handleSearch = useMemo(
    () => debounce((searchQuery: string) => {
      setSearchQuery(searchQuery);
    }, 300),
    [setSearchQuery]
  );

  useEffect(() => {
    handleSearch(query);
  }, [query, handleSearch]);

  return (
    <input
      value={query}
      onChange={(e) => setQuery(e.target.value)}
      placeholder="Search assets..."
    />
  );
}
```
