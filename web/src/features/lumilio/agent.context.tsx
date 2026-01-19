/**
 * Agent Context Provider
 * Provides state management for the AI chat feature
 */

import React, { createContext, useContext, ReactNode, useReducer } from "react";
import type { AgentContextValue } from "./types";
import { agentReducer, initialState } from "./agent.reducer";

const AgentContext = createContext<AgentContextValue | undefined>(undefined);

export interface AgentProviderProps {
  children: ReactNode;
}

/**
 * Provider component for agent state management
 */
export const AgentProvider: React.FC<AgentProviderProps> = ({ children }) => {
  const [state, dispatch] = useReducer(agentReducer, initialState);

  const value: AgentContextValue = {
    state,
    dispatch,
  };

  return <AgentContext.Provider value={value}>{children}</AgentContext.Provider>;
};

/**
 * Hook to access agent context
 * @throws Error if used outside of AgentProvider
 */
export const useAgentContext = (): AgentContextValue => {
  const context = useContext(AgentContext);
  if (!context) {
    throw new Error("useAgentContext must be used within an AgentProvider");
  }
  return context;
};
