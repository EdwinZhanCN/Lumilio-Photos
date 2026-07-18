import { useCallback, useEffect, useMemo, useRef } from "react";
import { useFilterActions, useFilterState } from "../../../state/selectors";
import type { FilterDTO } from "../../browse/FilterTool/types";
import { filterDTOToPayload, filtersToDTO } from "./filterState";
import type { AssetsPageHeaderProps } from "./types";

export function useAssetsPageHeaderFilters(
  onFiltersChange: AssetsPageHeaderProps["onFiltersChange"],
) {
  const filters = useFilterState();
  const { batchUpdateFilters } = useFilterActions();

  const inboundDTO = useMemo(() => filtersToDTO(filters), [filters]);
  const inboundHash = useMemo(() => JSON.stringify(inboundDTO || {}), [inboundDTO]);
  const onFiltersChangeRef = useRef(onFiltersChange);

  useEffect(() => {
    onFiltersChangeRef.current = onFiltersChange;
  });

  const handleFiltersChange = useCallback(
    (newFilters: FilterDTO) => {
      const nextHash = JSON.stringify(newFilters || {});
      if (nextHash === inboundHash) {
        return;
      }

      batchUpdateFilters(filterDTOToPayload(newFilters));
      onFiltersChangeRef.current?.(newFilters);
    },
    [batchUpdateFilters, inboundHash],
  );

  return { inboundDTO, handleFiltersChange };
}
