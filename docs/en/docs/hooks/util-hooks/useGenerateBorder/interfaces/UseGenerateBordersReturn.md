[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateBorder](../index.md) / UseGenerateBordersReturn

# Interface: UseGenerateBordersReturn

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:45](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/130ee90cd12122a0d6ac1018d6d9ee450974d021/web/src/hooks/util-hooks/useGenerateBorder.tsx#L45)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:55](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/130ee90cd12122a0d6ac1018d6d9ee450974d021/web/src/hooks/util-hooks/useGenerateBorder.tsx#L55)

#### Returns

`void`

***

### generateBorders()

> **generateBorders**: (`files`, `option`, `param`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:50](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/130ee90cd12122a0d6ac1018d6d9ee450974d021/web/src/hooks/util-hooks/useGenerateBorder.tsx#L50)

#### Parameters

##### files

`File`[]

##### option

[`BorderOptions`](../type-aliases/BorderOptions.md)

##### param

\{ `b`: `number`; `border_width`: `number`; `g`: `number`; `jpeg_quality`: `number`; `r`: `number`; \} | \{ `blur_sigma`: `number`; `brightness_adjustment`: `number`; `corner_radius`: `number`; `jpeg_quality`: `number`; \} | \{ `jpeg_quality`: `number`; `strength`: `number`; \}

#### Returns

`Promise`\<`void`\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:46](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/130ee90cd12122a0d6ac1018d6d9ee450974d021/web/src/hooks/util-hooks/useGenerateBorder.tsx#L46)

***

### processedImages

> **processedImages**: [`ProcessedImageMap`](../type-aliases/ProcessedImageMap.md)

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:47](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/130ee90cd12122a0d6ac1018d6d9ee450974d021/web/src/hooks/util-hooks/useGenerateBorder.tsx#L47)

***

### progress

> **progress**: [`BorderGenerationProgress`](../type-aliases/BorderGenerationProgress.md)

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:48](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/130ee90cd12122a0d6ac1018d6d9ee450974d021/web/src/hooks/util-hooks/useGenerateBorder.tsx#L48)

***

### setProcessedImages

> **setProcessedImages**: `Dispatch`\<`SetStateAction`\<[`ProcessedImageMap`](../type-aliases/ProcessedImageMap.md)\>\>

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:49](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/130ee90cd12122a0d6ac1018d6d9ee450974d021/web/src/hooks/util-hooks/useGenerateBorder.tsx#L49)
