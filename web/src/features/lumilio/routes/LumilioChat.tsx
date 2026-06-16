import { AgentBoard } from "../components/Board/AgentBoard";
import { ChatDock } from "../components/Chat/ChatDock";

const LumilioChatPage = () => {
  return (
    <div className="relative h-full bg-base-100">
      <AgentBoard />
      <ChatDock variant="embedded" />
    </div>
  );
};

export default LumilioChatPage;
