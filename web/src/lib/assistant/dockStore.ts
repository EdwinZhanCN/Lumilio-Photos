import { create } from "zustand";

interface DockStore {
  /** User-toggled collapse; null = use route default. */
  collapsedOverride: boolean | null;
  setCollapsed: (collapsed: boolean | null) => void;
  /** Whether the agent is currently generating a response. */
  isGenerating: boolean;
  setGenerating: (generating: boolean) => void;
}

export const useDockStore = create<DockStore>((set) => ({
  collapsedOverride: null,
  setCollapsed: (collapsed) => set({ collapsedOverride: collapsed }),
  isGenerating: false,
  setGenerating: (generating) => set({ isGenerating: generating }),
}));
