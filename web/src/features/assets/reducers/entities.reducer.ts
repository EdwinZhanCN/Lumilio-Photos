import { AssetsAction, EntitiesState, EntityMeta } from "../types";
import { Asset } from "@/services";

export const initialEntitiesState: EntitiesState = {
  assets: {},
  meta: {},
};

export const entitiesReducer = (
  state: EntitiesState = initialEntitiesState,
  action: AssetsAction,
): EntitiesState => {
  switch (action.type) {
    case "SET_ENTITY": {
      const { assetId, asset, meta } = action.payload;
      return {
        ...state,
        assets: {
          ...state.assets,
          [assetId]: asset,
        },
        meta: {
          ...state.meta,
          [assetId]: {
            lastUpdated: Date.now(),
            ...meta,
          },
        },
      };
    }

    case "UPDATE_ENTITY": {
      const { assetId, updates, meta } = action.payload;
      const existingAsset = state.assets[assetId];
      if (!existingAsset) {
        return state;
      }

      return {
        ...state,
        assets: {
          ...state.assets,
          [assetId]: {
            ...existingAsset,
            ...updates,
          },
        },
        meta: {
          ...state.meta,
          [assetId]: {
            ...state.meta[assetId],
            lastUpdated: Date.now(),
            ...meta,
          },
        },
      };
    }

    case "DELETE_ENTITY": {
      const { assetId } = action.payload;
      const newAssets = { ...state.assets };
      const newMeta = { ...state.meta };
      delete newAssets[assetId];
      delete newMeta[assetId];

      return {
        ...state,
        assets: newAssets,
        meta: newMeta,
      };
    }

    case "BATCH_SET_ENTITIES": {
      const { assets, meta = {} } = action.payload;
      const newAssets = { ...state.assets };
      const newMeta = { ...state.meta };
      const now = Date.now();

      assets.forEach((asset) => {
        if (asset.asset_id) {
          newAssets[asset.asset_id] = asset;
          newMeta[asset.asset_id] = {
            lastUpdated: now,
            ...meta[asset.asset_id],
          };
        }
      });

      return {
        ...state,
        assets: newAssets,
        meta: newMeta,
      };
    }

    default:
      return state;
  }
};

// Selectors
export const selectAsset = (state: EntitiesState, assetId: string): Asset | undefined => {
  return state.assets[assetId];
};

export const selectAssets = (state: EntitiesState, assetIds: string[]): Asset[] => {
  return assetIds
    .map((id) => state.assets[id])
    .filter((asset): asset is Asset => asset !== undefined);
};

export const selectAssetMeta = (state: EntitiesState, assetId: string): EntityMeta | undefined => {
  return state.meta[assetId];
};

export const selectAllAssets = (state: EntitiesState): Asset[] => {
  return Object.values(state.assets);
};
