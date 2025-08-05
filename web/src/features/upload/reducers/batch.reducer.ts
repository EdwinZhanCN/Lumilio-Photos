import { UploadAction, BatchUploadState } from "../types";

export const batchReducer = (
  state: BatchUploadState,
  action: UploadAction,
): BatchUploadState => {
  switch (action.type) {
    case "SET_BATCH_FILES": {
      const newBatchCount = action.payload.files.length;
      return {
        ...state,
        files: action.payload.files,
        count: newBatchCount,
      };
    }
    case "CLEAR_BATCH_FILES":
      return {
        ...state,
        files: [],
        count: 0,
      };
    default:
      return state;
  }
};
