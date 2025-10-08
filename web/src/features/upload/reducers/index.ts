import { UploadAction, UploadState } from "../types";

export const initialState: UploadState = {
  files: [],
  previews: [],
  isDragging: false,
};

export const uploadReducer = (
  state: UploadState,
  action: UploadAction,
): UploadState => {
  switch (action.type) {
    case "SET_DRAGGING":
      return { ...state, isDragging: action.payload };

    case "ADD_FILES": {
      const newFiles = [...state.files, ...action.payload.files];
      const newPreviews = [...state.previews, ...action.payload.previews];

      return {
        ...state,
        files: newFiles,
        previews: newPreviews,
      };
    }

    case "UPDATE_PREVIEW_URLS": {
      const { startIndex, urls } = action.payload;
      const newPreviews = [...state.previews];

      urls.forEach((url, index) => {
        const targetIndex = startIndex + index;
        if (targetIndex < newPreviews.length) {
          // Revoke old URL if it exists
          if (newPreviews[targetIndex]) {
            URL.revokeObjectURL(newPreviews[targetIndex]);
          }
          newPreviews[targetIndex] = url;
        }
      });

      return { ...state, previews: newPreviews };
    }

    case "CLEAR_FILES":
      // Revoke all preview URLs to prevent memory leaks
      state.previews.forEach((url) => {
        if (url) URL.revokeObjectURL(url);
      });
      return initialState;

    default:
      return state;
  }
};
