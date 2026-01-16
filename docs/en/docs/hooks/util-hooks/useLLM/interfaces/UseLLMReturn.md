[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useLLM](../index.md) / UseLLMReturn

# Interface: UseLLMReturn

Defined in: [hooks/util-hooks/useLLM.tsx:32](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L32)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useLLM.tsx:43](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L43)

#### Returns

`void`

***

### clearConversation()

> **clearConversation**: () => `void`

Defined in: [hooks/util-hooks/useLLM.tsx:42](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L42)

#### Returns

`void`

***

### conversation

> **conversation**: [`LLMMessage`](LLMMessage.md)[]

Defined in: [hooks/util-hooks/useLLM.tsx:36](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L36)

***

### currentModelId

> **currentModelId**: `null` \| `string`

Defined in: [hooks/util-hooks/useLLM.tsx:37](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L37)

***

### generateAnswer()

> **generateAnswer**: (`userInput`, `options?`) => `Promise`\<`undefined` \| `string`\>

Defined in: [hooks/util-hooks/useLLM.tsx:38](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L38)

#### Parameters

##### userInput

`string`

##### options?

[`LLMOptions`](LLMOptions.md)

#### Returns

`Promise`\<`undefined` \| `string`\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useLLM.tsx:34](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L34)

***

### isInitializing

> **isInitializing**: `boolean`

Defined in: [hooks/util-hooks/useLLM.tsx:33](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L33)

***

### progress

> **progress**: `null` \| [`LLMProgress`](LLMProgress.md)

Defined in: [hooks/util-hooks/useLLM.tsx:35](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L35)

***

### setSystemPrompt()

> **setSystemPrompt**: (`prompt`) => `void`

Defined in: [hooks/util-hooks/useLLM.tsx:44](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/hooks/util-hooks/useLLM.tsx#L44)

#### Parameters

##### prompt

`string`

#### Returns

`void`
