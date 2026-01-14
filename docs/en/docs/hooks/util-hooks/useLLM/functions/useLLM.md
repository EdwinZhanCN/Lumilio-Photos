[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useLLM](../index.md) / useLLM

# Function: useLLM()

> **useLLM**(): [`UseLLMReturn`](../interfaces/UseLLMReturn.md)

Defined in: [hooks/util-hooks/useLLM.tsx:55](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/2b82469e35f05bd0a6f84f5b5214775da7893f65/web/src/hooks/util-hooks/useLLM.tsx#L55)

Custom hook for LLM interactions using the shared web worker client.
It manages conversation state, streaming responses, and progress tracking.
This hook must be used within a component tree wrapped by `<WorkerProvider />`.

## Returns

[`UseLLMReturn`](../interfaces/UseLLMReturn.md)

Hook state and actions for LLM interaction.

## Author

Edwin Zhan

## Since

1.1.0
