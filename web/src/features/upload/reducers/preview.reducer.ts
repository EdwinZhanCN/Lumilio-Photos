import { UploadAction, PreviewUploadState } from "../types";

export const previewReducer = (
  state: PreviewUploadState,
  action: UploadAction,
): PreviewUploadState => {
  switch (action.type) {
    case "SET_PREVIEW_FILES": {
      // Clean up old preview URLs before setting new ones
      state.previews.forEach((url) => {
        if (url) URL.revokeObjectURL(url);
      });
      const newPreviewCount = action.payload.files.length;
      return {
        ...state,
        files: action.payload.files,
        previews: action.payload.previews,
        count: newPreviewCount,
      };
    }
    case "UPDATE_PREVIEW_URLS": {
      const { startIndex, urls } = action.payload;
      const newPreviews = [...state.previews];
      urls.forEach((url, index) => {
        if (startIndex + index < newPreviews.length) {
          newPreviews[startIndex + index] = url;
        }
      });
      return {
        ...state,
        previews: newPreviews,
      };
    }
    case "CLEAR_PREVIEW_FILES":
      state.previews.forEach((url) => {
        if (url) URL.revokeObjectURL(url);
      });
      return {
        ...state,
        files: [],
        previews: [],
        count: 0,
      };
    default:
      return state;
  }
};
