// src/features/lumilio/routes/LumilioChat.tsx
import React, { useState } from "react";
import { Link } from "react-router-dom";
import { LumilioChatProvider } from "../LumilioChatProvider";

import { useLumilioChat } from "../hooks/useLumilioChat";
import { LumilioInput, LumilioMessages } from "../components/LumilioChat";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n.tsx";

const ChatInterface: React.FC = () => {
  const { t } = useI18n();
  const { state, sendMessage, resumeConversation, dispatch } = useLumilioChat();
  const [activeTool, setActiveTool] = useState<string | null>(null);
  const { capabilities } = useCapabilities(5000);

  const rootCauseInterrupt = state.interrupt?.InterruptContexts?.find(
    (ctx) => ctx.IsRootCause,
  );
  const agentDisabledReason =
    capabilities && !capabilities.llm.agentEnabled
      ? t("lumilio.agent.disabled")
      : capabilities && !capabilities.llm.configured
        ? t("lumilio.agent.notConfigured")
        : null;

  const handleSubmit = (query: string) => {
    const toolNames = activeTool ? [activeTool] : [];
    sendMessage(query, toolNames);
    setActiveTool(null);
  };

  const handleConfirmation = (approved: boolean) => {
    if (!rootCauseInterrupt) {
      console.error("No root cause found in interrupt");
      dispatch({ type: "CLEAR_INTERRUPT" });
      return;
    }
    const targets = { [rootCauseInterrupt.ID]: { approved } };
    resumeConversation(targets);
  };

  return (
    <div className="flex flex-col h-full bg-base-100">
      {/* Scrollable Message Area */}
      <div className="flex-1 overflow-y-auto">
        {state.conversation.length === 0 && !state.isGenerating ? (
          <div className="flex h-full items-center justify-center">
            <div className="text-center text-base-content/50">
              <p>{t("lumilio.chat.empty")}</p>
            </div>
          </div>
        ) : (
          <LumilioMessages
            conversation={state.conversation}
            isGenerating={state.isGenerating}
          />
        )}
      </div>

      {/* Fixed Input Area at the bottom */}
      <div className="shrink-0 border-t border-base-300">
        {agentDisabledReason && (
          <div className="px-4 py-3 border-b border-base-300 bg-warning/10 text-sm text-base-content/80">
            <span>{agentDisabledReason}</span>{" "}
            <Link className="underline hover:opacity-80" to="/settings?tab=ai">
              {t("lumilio.chat.openAiSettings")}
            </Link>
          </div>
        )}

        {rootCauseInterrupt?.Info && (
          <div className="p-4 border-b border-base-300 bg-warning/10">
            <h4 className="font-bold text-warning">
              {t("lumilio.chat.confirmation.title")}
            </h4>
            <p className="text-sm my-2">{rootCauseInterrupt.Info.message}</p>
            <div className="flex gap-2">
              <button
                className="px-3 py-1.5 text-sm font-medium rounded-lg bg-success text-success-content hover:brightness-90 transition-all"
                onClick={() => handleConfirmation(true)}
              >
                {t("lumilio.chat.confirmation.confirm")}
              </button>
              <button
                className="px-3 py-1.5 text-sm font-medium rounded-lg bg-error text-error-content hover:brightness-90 transition-all"
                onClick={() => handleConfirmation(false)}
              >
                {t("common.cancel")}
              </button>
            </div>
          </div>
        )}

        <LumilioInput
          isGenerating={state.isGenerating}
          isInitializing={state.connection.status === "connecting"}
          commands={state.tools.available}
          disabled={Boolean(agentDisabledReason)}
          disabledHint={agentDisabledReason ?? undefined}
          onSubmit={handleSubmit}
        />
      </div>
    </div>
  );
};

const LumilioChatPage: React.FC = () => {
  return (
    <LumilioChatProvider>
      <ChatInterface />
    </LumilioChatProvider>
  );
};

export default LumilioChatPage;
