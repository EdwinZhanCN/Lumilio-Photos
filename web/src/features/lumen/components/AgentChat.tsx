/**
 * Agent Chat Component
 * Main chat interface using custom runtime provider
 */

import { forwardRef, useImperativeHandle } from "react";
import {
  AgentRuntimeProvider,
  useRuntime,
} from "../runtime/AgentRuntimeProvider";
import { LumenChat } from "./LumenChat";

export interface AgentChatProps {
  className?: string;
}

export interface AgentChatRef {
  clear: () => void;
}

// Internal component that has access to the runtime
const AgentChatInternal = forwardRef<AgentChatRef, AgentChatProps>(
  ({ className }, ref) => {
    const { clear } = useRuntime();

    // Expose the clear method through the ref
    useImperativeHandle(
      ref,
      () => ({
        clear,
      }),
      [clear],
    );

    return (
      <div className={className}>
        <LumenChat />
      </div>
    );
  },
);

AgentChatInternal.displayName = "AgentChatInternal";

/**
 * Agent Chat Component with:
 * - AgentRuntimeProvider: Custom runtime provider that integrates with our backend agent service
 * - LumenChat: Custom chat interface that uses the runtime
 * - Forwarded ref: Exposes clear method to parent components
 */
export const AgentChat = forwardRef<AgentChatRef, AgentChatProps>(
  ({ className }, ref) => {
    return (
      <AgentRuntimeProvider>
        <AgentChatInternal className={className} ref={ref} />
      </AgentRuntimeProvider>
    );
  },
);

AgentChat.displayName = "AgentChat";
