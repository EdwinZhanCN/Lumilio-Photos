import { Blocks } from "lucide-react";
import { Link } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { AgentBoard } from "../components/Board/AgentBoard";
import { ChatDock } from "../components/Chat/ChatDock";

const LumilioChatPage = () => {
  const { t } = useI18n();

  return (
    <div className="relative h-full bg-base-100">
      <div className="absolute right-4 top-4 z-20">
        <Link className="btn btn-sm btn-ghost bg-base-100/85 shadow-sm" to="/lumilio/widgets">
          <Blocks size={16} />
          {t("lumilio.widgetLibrary.open", "Widget Library")}
        </Link>
      </div>
      <AgentBoard />
      <ChatDock variant="embedded" />
    </div>
  );
};

export default LumilioChatPage;
