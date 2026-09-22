import { useEventRebuild, useEventRebuildStatus } from "@/features/events";
import { useRebuildPeopleClusters } from "@/features/people";
import StoragePanel from "@/features/repositories/routes/StoragePanel";

export default function StoragePage() {
  const { rebuildPeople, isRebuilding: isRebuildingPeople } = useRebuildPeopleClusters();
  const { rebuild: rebuildEvents, isRebuilding: isRebuildingEvents } = useEventRebuild();
  useEventRebuildStatus();

  return (
    <StoragePanel
      maintenance={{
        rebuildEvents: () => void rebuildEvents(),
        rebuildPeople: () => void rebuildPeople(),
        isRebuildingEvents,
        isRebuildingPeople,
      }}
    />
  );
}
