import { create } from "zustand";

interface DockStore {
  /** User-toggled collapse; null = use route default. */
  collapsedOverride: boolean | null;
  setCollapsed: (collapsed: boolean | null) => void;
}

export const useDockStore = create<DockStore>((set) => ({
  collapsedOverride: null,
  setCollapsed: (collapsed) => set({ collapsedOverride: collapsed }),
}));
