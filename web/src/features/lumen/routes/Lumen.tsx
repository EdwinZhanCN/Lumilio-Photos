import { AgentChat } from "../components/AgentChat";
import { PageHeader } from "@/components/PageHeader";
import { SparklesIcon } from "@heroicons/react/24/outline";
import { useRef } from "react";

export function Lumen() {
  const agentChatRef = useRef<{ clear: () => void }>(null);

  const handleNewChat = () => {
    if (agentChatRef.current) {
      agentChatRef.current.clear();
    }
  };

  return (
    <div className="h-full flex flex-col">
      <PageHeader
        title="Lumen Agent"
        icon={<SparklesIcon className="w-6 h-6 text-primary" />}
      >
        <button
          className="btn btn-sm btn-soft btn-info"
          onClick={handleNewChat}
        >
          New
        </button>
      </PageHeader>
      <div className="flex-1 overflow-hidden">
        <AgentChat className="h-full" ref={agentChatRef} />
      </div>
    </div>
  );
}
