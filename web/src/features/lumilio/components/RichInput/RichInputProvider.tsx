// Lumilio-Photos/web/src/features/lumilio/rich-input/RichInputProvider.tsx
import React, { createContext, useReducer, ReactNode } from "react";
import { RichInputContextValue } from "./types";
import { RichInputReducer, initialState } from "./rich.reducer";

/**
 * RichInput Context
 *
 * 提供 RichInput 组件的状态和 dispatch 方法
 */
export const RichInputContext = createContext<
  RichInputContextValue | undefined
>(undefined);

/**
 * RichInputProvider 组件
 *
 * 为 RichInput 及其子组件提供状态管理
 *
 * @example
 * ```tsx
 * <RichInputProvider>
 *   <YourComponent />
 * </RichInputProvider>
 * ```
 */
export const RichInputProvider = ({ children }: { children: ReactNode }) => {
  const [state, dispatch] = useReducer(RichInputReducer, initialState);

  const value: RichInputContextValue = {
    state,
    dispatch,
  };

  return (
    <RichInputContext.Provider value={value}>
      {children}
    </RichInputContext.Provider>
  );
};

/**
 * useRichInput Hook
 *
 * 用于在组件中访问 RichInput 的状态和 dispatch 方法
 *
 * @throws 如果在 RichInputProvider 外部使用会抛出错误
 *
 * @example
 * ```tsx
 * const { state, dispatch } = useRichInput();
 *
 * // 访问状态
 * console.log(state.phase);
 * console.log(state.payload);
 *
 * // 分发 action
 * dispatch({ type: "SET_PHASE", payload: "SELECT_TYPE" });
 * ```
 */
export const useRichInput = (): RichInputContextValue => {
  const context = React.useContext(RichInputContext);

  if (context === undefined) {
    throw new Error(
      "useRichInput must be used within a RichInputProvider. " +
        "Wrap your component tree with <RichInputProvider>.",
    );
  }

  return context;
};
