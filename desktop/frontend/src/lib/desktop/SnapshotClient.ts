import { Events } from "@wailsio/runtime";
import { DesktopService } from "../../../bindings/desktop/internal/control/index.js";
import type { DesktopSnapshot } from "../../../bindings/desktop/internal/control/dto/models.js";
import type { SnapshotNotice } from "./events";

type SnapshotListener = (snapshot: DesktopSnapshot) => void;

export class SnapshotClient {
  private snapshot: DesktopSnapshot | null = null;
  private listeners = new Set<SnapshotListener>();
  private stopEvents: (() => void) | null = null;
  private highest: SnapshotNotice | null = null;
  private readQueue: Promise<void> = Promise.resolve();

  subscribe(listener: SnapshotListener) {
    this.listeners.add(listener);
    if (this.snapshot) listener(this.snapshot);
    return () => this.listeners.delete(listener);
  }

  async start() {
    this.stopEvents = Events.On("desktop:snapshot-changed", (event) => {
      const notice = event.data;
      if (!notice || !notice.instanceID || typeof notice.revision !== "number") return;
      if (!this.isNewer(notice)) return;
      this.highest = notice;
      this.enqueueRead();
    });

    const highestSeenDuringSubscription = this.highest;
    const first = await DesktopService.GetSnapshot();
    this.accept(first);
    if (highestSeenDuringSubscription && highestSeenDuringSubscription.instanceID === first.instanceID && highestSeenDuringSubscription.revision > first.revision) {
      await this.enqueueRead();
    }
  }

  close() {
    this.stopEvents?.();
    this.stopEvents = null;
    this.listeners.clear();
  }

  private enqueueRead() {
    this.readQueue = this.readQueue.then(async () => {
      const next = await DesktopService.GetSnapshot();
      // Compare against the last *accepted* snapshot, not the highest notice:
      // start() can accept a stale initial read after a newer notice arrived,
      // and an equal-revision follow-up would otherwise be dropped, freezing
      // the UI on an old snapshot forever.
      if (!this.snapshot || next.revision > this.snapshot.revision) this.accept(next);
    }).catch(() => undefined);
    return this.readQueue;
  }

  private accept(next: DesktopSnapshot) {
    this.snapshot = next;
    if (!this.highest || this.highest.instanceID !== next.instanceID || next.revision >= this.highest.revision) {
      this.highest = { instanceID: next.instanceID, revision: next.revision };
    }
    for (const listener of this.listeners) listener(next);
  }

  private isNewer(next: SnapshotNotice) {
    if (!this.highest || this.highest.instanceID !== next.instanceID) return true;
    return next.revision > this.highest.revision;
  }
}
