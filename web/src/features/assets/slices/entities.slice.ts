import { StateCreator } from "zustand";
import { Asset } from "@/services";
import { EntitiesState, EntityMeta } from "../types/assets.type";

export interface EntitiesSlice {
  entities: EntitiesState;
  setEntity: (
    assetId: string,
    asset: Asset,
    meta?: Partial<EntityMeta>,
  ) => void;
  updateEntity: (
    assetId: string,
    updates: Partial<Asset>,
    meta?: Partial<EntityMeta>,
  ) => void;
  deleteEntity: (assetId: string) => void;
  batchSetEntities: (
    assets: Asset[],
    meta?: Record<string, Partial<EntityMeta>>,
  ) => void;
}

export const createEntitiesSlice: StateCreator<
  EntitiesSlice,
  [["zustand/immer", never]],
  [],
  EntitiesSlice
> = (set) => ({
  entities: {
    assets: {},
    meta: {},
  },

  setEntity: (assetId, asset, meta) =>
    set((state) => {
      state.entities.assets[assetId] = asset;
      state.entities.meta[assetId] = {
        lastUpdated: Date.now(),
        ...meta,
      };
    }),

  updateEntity: (assetId, updates, meta) =>
    set((state) => {
      const existingAsset = state.entities.assets[assetId];
      if (existingAsset) {
        Object.assign(existingAsset, updates);
        state.entities.meta[assetId] = {
          ...state.entities.meta[assetId],
          lastUpdated: Date.now(),
          ...meta,
        };
      }
    }),

  deleteEntity: (assetId) =>
    set((state) => {
      delete state.entities.assets[assetId];
      delete state.entities.meta[assetId];
    }),

  batchSetEntities: (assets, meta = {}) =>
    set((state) => {
      const now = Date.now();
      assets.forEach((asset) => {
        if (asset.asset_id) {
          state.entities.assets[asset.asset_id] = asset;
          state.entities.meta[asset.asset_id] = {
            lastUpdated: now,
            ...meta[asset.asset_id],
          };
        }
      });
    }),
});

// Selectors - work with both EntitiesSlice (store) and EntitiesState (legacy context)
type EntitiesInput = EntitiesSlice | EntitiesState;

// Helper to normalize input
const getEntitiesState = (input: EntitiesInput): EntitiesState => {
  if ('entities' in input && input.entities && 'assets' in input.entities) {
    return input.entities;
  }
  return input as EntitiesState;
};

export const selectAsset = (
  input: EntitiesInput,
  assetId: string,
): Asset | undefined => {
  const state = getEntitiesState(input);
  return state.assets[assetId];
};

export const selectAssets = (
  input: EntitiesInput,
  assetIds: string[],
): Asset[] => {
  const state = getEntitiesState(input);
  return assetIds
    .map((id) => state.assets[id])
    .filter((asset): asset is Asset => asset !== undefined);
};

export const selectAssetMeta = (
  input: EntitiesInput,
  assetId: string,
): EntityMeta | undefined => {
  const state = getEntitiesState(input);
  return state.meta[assetId];
};

export const selectAllAssets = (input: EntitiesInput): Asset[] => {
  const state = getEntitiesState(input);
  return Object.values(state.assets);
};

