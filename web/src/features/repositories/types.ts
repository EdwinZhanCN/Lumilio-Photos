export type RepositoryOption = {
  id: string;
  name: string;
  path: string;
  role: string;
  /**
   * Reachability of the repository's on-disk location. An "offline" repository
   * stays selectable as a browse filter but must not be offered as an upload
   * target: its drive is elsewhere, its assets are not gone.
   */
  status: RepositoryStatus;
  isPrimary: boolean;
};

export type RepositoryStatus = "active" | "scanning" | "error" | "offline";
