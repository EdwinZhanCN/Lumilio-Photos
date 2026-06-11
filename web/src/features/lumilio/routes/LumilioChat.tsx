import React from "react";
import { Link, useLocation } from "react-router-dom";
import { LayoutDashboard, MessageCircle, SquarePen } from "lucide-react";
import { useLumilioChatStore } from "../state/chatStore";
import { ChatMessages } from "../components/Chat/ChatMessages";
import { LumilioInput } from "../components/LumilioChat";
import { AgentBoard } from "../components/Board/AgentBoard";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n.tsx";

/** Tab bar shared by the chat and board views. */
function LumilioTabs() {
  const { t } = useI18n();
  const { pathname } = useLocation();
  const isBoard = pathname.endsWith("/board");

  const tabClass = (active: boolean) =>
    `inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-full transition-colors ${
      active
        ? "bg-base-200 text-base-content"
        : "text-base-content/50 hover:text-base-content"
    }`;

  return (
    <div className="flex items-center gap-1 px-4 py-2 border-b border-base-300 shrink-0">
      <Link to="/lumilio" className={tabClass(!isBoard)}>
        <MessageCircle size={15} strokeWidth={1.5} />
        {t("lumilio.tabs.chat", "Chat")}
      </Link>
      <Link to="/lumilio/board" className={tabClass(isBoard)}>
        <LayoutDashboard size={15} strokeWidth={1.5} />
        {t("lumilio.tabs.board", "Board")}
      </Link>
    </div>
  );
}

const ChatView: React.FC = () => {
  const { t } = useI18n();
  const messages = useLumilioChatStore((s) => s.messages);
  const isGenerating = useLumilioChatStore((s) => s.isGenerating);
  const connectionError = useLumilioChatStore((s) => s.connectionError);
  const sendMessage = useLumilioChatStore((s) => s.sendMessage);
  const newConversation = useLumilioChatStore((s) => s.newConversation);
  const { capabilities } = useCapabilities(5000);

  const agentDisabledReason =
    capabilities && !capabilities.llm.agentEnabled
      ? t("lumilio.agent.disabled")
      : capabilities && !capabilities.llm.configured
        ? t("lumilio.agent.notConfigured")
        : null;

  return (
    <>
      {/* Scrollable message area */}
      <div className="flex-1 overflow-y-auto">
        {messages.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <div className="text-center text-base-content/50">
              <p>{t("lumilio.chat.empty")}</p>
            </div>
          </div>
        ) : (
          <ChatMessages messages={messages} isGenerating={isGenerating} />
        )}
      </div>

      {/* Fixed input area */}
      <div className="shrink-0 border-t border-base-300">
        {agentDisabledReason && (
          <div className="px-4 py-3 border-b border-base-300 bg-warning/10 text-sm text-base-content/80">
            <span>{agentDisabledReason}</span>{" "}
            <Link className="underline hover:opacity-80" to="/settings?tab=ai">
              {t("lumilio.chat.openAiSettings")}
            </Link>
          </div>
        )}

        {connectionError && (
          <div className="px-4 py-2 border-b border-base-300 bg-error/10 text-sm text-error">
            {connectionError}
          </div>
        )}

        <div className="relative">
          {messages.length > 0 && (
            <button
              className="absolute right-4 -top-10 inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border border-base-300 bg-base-100 text-xs text-base-content/60 hover:text-base-content transition-colors"
              onClick={newConversation}
              title={t("lumilio.chat.newConversation", "New conversation")}
            >
              <SquarePen size={13} strokeWidth={1.5} />
              {t("lumilio.chat.newConversation", "New conversation")}
            </button>
          )}
          <LumilioInput
            isGenerating={isGenerating}
            isInitializing={false}
            disabled={Boolean(agentDisabledReason)}
            disabledHint={agentDisabledReason ?? undefined}
            onSubmit={(value) => void sendMessage(value)}
          />
        </div>
      </div>
    </>
  );
};

const LumilioChatPage: React.FC<{ view?: "chat" | "board" }> = ({
  view = "chat",
}) => {
  return (
    <div className="flex flex-col h-full bg-base-100">
      <LumilioTabs />
      {view === "board" ? (
        <div className="flex-1 min-h-0">
          <AgentBoard />
        </div>
      ) : (
        <ChatView />
      )}
    </div>
  );
};

export default LumilioChatPage;
