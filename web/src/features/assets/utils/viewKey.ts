import { AssetViewDefinition } from "@/features/assets";

export const generateViewKey = (definition: AssetViewDefinition): string => {
  if (definition.key) {
    return definition.key;
  }

  const normalizedDef = {
    types: definition.types ? [...definition.types].sort() : [],
    filter: definition.filter || {},
    inheritGlobalFilter: definition.inheritGlobalFilter ?? true,
    search: definition.search,
    groupBy: definition.groupBy || "date",
    sort: definition.sort || { field: "taken_time", direction: "desc" },
    pageSize: definition.pageSize || 50,
    pagination: definition.pagination || "cursor",
  };

  const hashInput = JSON.stringify(normalizedDef);

  let hash = 0;
  for (let i = 0; i < hashInput.length; i++) {
    const char = hashInput.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash = hash & hash;
  }

  return `view_${Math.abs(hash).toString(36)}`;
};
