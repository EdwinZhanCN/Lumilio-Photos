import { AgentBoard } from "../board/AgentBoard";
import { ChatDock } from "../chat/ChatDock";

const LumilioChatPage = () => {
  return (
    <div className="relative h-full bg-base-100">
      <AgentBoard />
      <ChatDock variant="embedded" />
    </div>
  );
};

export default LumilioChatPage;
