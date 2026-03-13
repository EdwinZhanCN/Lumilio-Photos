[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateThumbnail](../index.md) / UseGenerateThumbnailReturn

# Interface: UseGenerateThumbnailReturn

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:25](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a511d476c5e6e4669dc25995dea857454d5e1bc5/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L25)

Represents the state and actions returned by the useGenerateThumbnail hook.

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:29](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a511d476c5e6e4669dc25995dea857454d5e1bc5/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L29)

#### Returns

`void`

***

### generatePreviews()

> **generatePreviews**: (`files`, `priority?`) => `Promise`\<`ThumbnailResult`[] \| `undefined`\>

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a511d476c5e6e4669dc25995dea857454d5e1bc5/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L28)

#### Parameters

##### files

`File`[]

##### priority?

`ProcessingPriority`

#### Returns

`Promise`\<`ThumbnailResult`[] \| `undefined`\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a511d476c5e6e4669dc25995dea857454d5e1bc5/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L26)

***

### progress

> **progress**: [`ThumbnailProgress`](ThumbnailProgress.md) \| `null`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:27](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a511d476c5e6e4669dc25995dea857454d5e1bc5/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L27)
