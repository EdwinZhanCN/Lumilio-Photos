import { UploadAction, UploadState } from "../types";
import { batchReducer } from "./batch.reducer";
import { previewReducer } from "./preview.reducer";

// This is the initial state for the entire upload feature.
export const initialState: UploadState = {
  preview: {
    files: [],
    previews: [],
    count: 0,
  },
  batch: {
    files: [],
    count: 0,
  },
  isDragging: false,
  totalFilesCount: 0,
  maxPreviewFiles: 30,
  maxBatchFiles: 50,
};

// This is the main reducer for the upload feature. It delegates actions
// to the appropriate sub-reducers.
export const uploadReducer = (
  state: UploadState,
  action: UploadAction,
): UploadState => {
  // Handle actions that affect the top-level state or have global effects.
  switch (action.type) {
    case "SET_DRAGGING":
      return { ...state, isDragging: action.payload };

    case "CLEAR_ALL_FILES":
      // Revoke any existing object URLs to prevent memory leaks.
      state.preview.previews.forEach((url) => {
        if (url) URL.revokeObjectURL(url);
      });
      // Reset the entire state to its initial condition.
      return { ...initialState };

    default: {
      // For all other actions, delegate to the sub-reducers.
      const newPreviewState = previewReducer(state.preview, action);
      const newBatchState = batchReducer(state.batch, action);

      // If the state from sub-reducers has changed, update the combined state.
      if (newPreviewState !== state.preview || newBatchState !== state.batch) {
        return {
          ...state,
          preview: newPreviewState,
          batch: newBatchState,
          // Recalculate the total file count based on the new sub-states.
          totalFilesCount: newPreviewState.count + newBatchState.count,
        };
      }

      // If no changes occurred in the sub-reducers, return the original state
      // to avoid unnecessary re-renders.
      return state;
    }
  }
};
