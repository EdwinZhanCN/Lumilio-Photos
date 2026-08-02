export interface SnapshotNotice {
  instanceID: string;
  revision: number;
}

declare module "@wailsio/runtime" {
  namespace Events {
    interface CustomEvents {
      "desktop:snapshot-changed": SnapshotNotice;
    }
  }
}
