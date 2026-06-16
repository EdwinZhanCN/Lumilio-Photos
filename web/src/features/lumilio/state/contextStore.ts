import { create } from "zustand";

export type ContextType = "selection" | "viewing";

export interface ContextContribution {
  id: string;
  type: ContextType;
  assetIds: string[];
  label: string;
}

interface ContextStore {
  contributions: Map<string, ContextContribution>;
  excluded: Set<string>;
  register: (contribution: ContextContribution) => void;
  unregister: (id: string) => void;
  exclude: (id: string) => void;
  include: (id: string) => void;
  /** Snapshot active contributions for send (respects exclusions). */
  snapshotForSend: () => ContextContribution[];
  clearExclusions: () => void;
}

export const useContextStore = create<ContextStore>((set, get) => ({
  contributions: new Map(),
  excluded: new Set(),

  register: (contribution) =>
    set((state) => {
      const next = new Map(state.contributions);
      next.set(contribution.id, contribution);
      return { contributions: next };
    }),

  unregister: (id) =>
    set((state) => {
      const next = new Map(state.contributions);
      next.delete(id);
      const excluded = new Set(state.excluded);
      excluded.delete(id);
      return { contributions: next, excluded };
    }),

  exclude: (id) =>
    set((state) => {
      const excluded = new Set(state.excluded);
      excluded.add(id);
      return { excluded };
    }),

  include: (id) =>
    set((state) => {
      const excluded = new Set(state.excluded);
      excluded.delete(id);
      return { excluded };
    }),

  snapshotForSend: () => {
    const { contributions, excluded } = get();
    return [...contributions.values()].filter((c) => !excluded.has(c.id) && c.assetIds.length > 0);
  },

  clearExclusions: () => set({ excluded: new Set() }),
}));
