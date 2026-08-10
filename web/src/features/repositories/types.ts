export type RepositoryOption = {
  id: string;
  name: string;
  path: string;
  role: string;
  rootId: string;
  /**
   * Reachability of the repository's on-disk location. Offline and invalid
   * repositories stay selectable as browse filters but are not upload targets.
   */
  reachability: RepositoryReachability;
  /** Work is orthogonal to reachability and never hides an unavailable state. */
  activity: RepositoryActivity;
  isPrimary: boolean;
};

export type RepositoryReachability =
  | "active"
  | "offline"
  | "identity_error"
  | "recovery_required"
  | "maintenance";

export type RepositoryActivity = "idle" | "scanning" | "importing" | "processing" | "paused";

export type RepositoryEffectiveState =
  | RepositoryReachability
  | "storage_location_offline"
  | "storage_location_error"
  | "storage_location_maintenance";
