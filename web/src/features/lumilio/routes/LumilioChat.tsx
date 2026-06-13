import { AgentBoard } from "../components/Board/AgentBoard";
import { ChatDock } from "../components/Chat/ChatDock";

const LumilioChatPage = () => {
  return (
    <div className="relative h-full bg-base-100">
      <AgentBoard />
      <ChatDock />
    </div>
  );
};

export default LumilioChatPage;
