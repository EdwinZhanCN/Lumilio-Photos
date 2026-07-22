export type RepositoryOption = {
  id: string;
  name: string;
  path: string;
  role: string;
  /**
   * Reachability of the repository's on-disk location. Offline and invalid
   * repositories stay selectable as browse filters but are not upload targets.
   */
  status: RepositoryStatus;
  isPrimary: boolean;
};

export type RepositoryStatus = "active" | "scanning" | "error" | "offline";
