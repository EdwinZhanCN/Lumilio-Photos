import { CollectionsAction, CollectionsState } from "./collections.type.ts";

export const initialState: CollectionsState = {
  selectedAlbumIds: [],
  isSelectionMode: false,
  isCreateModalOpen: false,
  isEditModalOpen: false,
  albumToEdit: null,
  isLoading: false,
  error: null,
};

export function collectionsReducer(
  state: CollectionsState,
  action: CollectionsAction,
): CollectionsState {
  switch (action.type) {
    case "TOGGLE_SELECTION_MODE":
      return {
        ...state,
        isSelectionMode: !state.isSelectionMode,
        selectedAlbumIds: [],
      };
    case "SELECT_ALBUM":
      return {
        ...state,
        selectedAlbumIds: [...state.selectedAlbumIds, action.payload],
      };
    case "DESELECT_ALBUM":
      return {
        ...state,
        selectedAlbumIds: state.selectedAlbumIds.filter(
          (id) => id !== action.payload,
        ),
      };
    case "CLEAR_SELECTION":
      return {
        ...state,
        selectedAlbumIds: [],
      };
    case "OPEN_CREATE_MODAL":
      return {
        ...state,
        isCreateModalOpen: true,
      };
    case "CLOSE_CREATE_MODAL":
      return {
        ...state,
        isCreateModalOpen: false,
      };
    case "OPEN_EDIT_MODAL":
      return {
        ...state,
        isEditModalOpen: true,
        albumToEdit: action.payload,
      };
    case "CLOSE_EDIT_MODAL":
      return {
        ...state,
        isEditModalOpen: false,
        albumToEdit: null,
      };
    case "SET_LOADING":
      return {
        ...state,
        isLoading: action.payload,
      };
    case "SET_ERROR":
      return {
        ...state,
        error: action.payload,
      };
    default:
      return state;
  }
}
