import { LumenChat } from "@/components/Lumen/LumenChat";
import { WorkerProvider } from "@/contexts/WorkerProvider";

export function Lumen() {
  return (
    <WorkerProvider preload={["llm"]}>
      <LumenChat />
    </WorkerProvider>
  );
}
