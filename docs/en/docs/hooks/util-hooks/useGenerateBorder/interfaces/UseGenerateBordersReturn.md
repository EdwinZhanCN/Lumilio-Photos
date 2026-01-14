[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateBorder](../index.md) / UseGenerateBordersReturn

# Interface: UseGenerateBordersReturn

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:48](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/fc7235ea377566f4cc2f62016aec0acb0e31f77b/web/src/hooks/util-hooks/useGenerateBorder.tsx#L48)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:59](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/fc7235ea377566f4cc2f62016aec0acb0e31f77b/web/src/hooks/util-hooks/useGenerateBorder.tsx#L59)

#### Returns

`void`

***

### generateBorders()

> **generateBorders**: (`files`, `option`, `param`, `priority?`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:53](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/fc7235ea377566f4cc2f62016aec0acb0e31f77b/web/src/hooks/util-hooks/useGenerateBorder.tsx#L53)

#### Parameters

##### files

`File`[]

##### option

[`BorderOptions`](../type-aliases/BorderOptions.md)

##### param

\{ `b`: `number`; `border_width`: `number`; `g`: `number`; `jpeg_quality`: `number`; `r`: `number`; \} | \{ `blur_sigma`: `number`; `brightness_adjustment`: `number`; `corner_radius`: `number`; `jpeg_quality`: `number`; \} | \{ `jpeg_quality`: `number`; `strength`: `number`; \}

##### priority?

[`ProcessingPriority`](../../../../utils/smartBatchSizing/enumerations/ProcessingPriority.md)

#### Returns

`Promise`\<`void`\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:49](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/fc7235ea377566f4cc2f62016aec0acb0e31f77b/web/src/hooks/util-hooks/useGenerateBorder.tsx#L49)

***

### processedImages

> **processedImages**: [`ProcessedImageMap`](../type-aliases/ProcessedImageMap.md)

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:50](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/fc7235ea377566f4cc2f62016aec0acb0e31f77b/web/src/hooks/util-hooks/useGenerateBorder.tsx#L50)

***

### progress

> **progress**: [`BorderGenerationProgress`](../type-aliases/BorderGenerationProgress.md)

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:51](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/fc7235ea377566f4cc2f62016aec0acb0e31f77b/web/src/hooks/util-hooks/useGenerateBorder.tsx#L51)

***

### setProcessedImages

> **setProcessedImages**: `Dispatch`\<`SetStateAction`\<[`ProcessedImageMap`](../type-aliases/ProcessedImageMap.md)\>\>

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:52](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/fc7235ea377566f4cc2f62016aec0acb0e31f77b/web/src/hooks/util-hooks/useGenerateBorder.tsx#L52)
