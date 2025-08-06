import { WorkerProvider } from "@/contexts/WorkerProvider";
import { LumenChat } from "../components/LumenChat";

export function Lumen() {
  return (
    <WorkerProvider preload={["llm"]}>
      <LumenChat />
    </WorkerProvider>
  );
}
