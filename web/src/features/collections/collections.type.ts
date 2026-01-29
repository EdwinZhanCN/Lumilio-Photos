import type { Album } from "@/lib/albums/types";

export interface CollectionsState {
  selectedAlbumIds: number[];
  isSelectionMode: boolean;
  isCreateModalOpen: boolean;
  isEditModalOpen: boolean;
  albumToEdit: Album | null;
  isLoading: boolean;
  error: string | null;
}

export type CollectionsAction =
  | { type: "TOGGLE_SELECTION_MODE" }
  | { type: "SELECT_ALBUM"; payload: number }
  | { type: "DESELECT_ALBUM"; payload: number }
  | { type: "CLEAR_SELECTION" }
  | { type: "OPEN_CREATE_MODAL" }
  | { type: "CLOSE_CREATE_MODAL" }
  | { type: "OPEN_EDIT_MODAL"; payload: Album }
  | { type: "CLOSE_EDIT_MODAL" }
  | { type: "SET_LOADING"; payload: boolean }
  | { type: "SET_ERROR"; payload: string | null };
