[lumilio-web](../../../../modules.md) / [hooks/wasm-hooks/useGenerateBorder](../index.md) / useGenerateBorders

# Function: useGenerateBorders()

> **useGenerateBorders**(`options`): `object`

Defined in: [hooks/wasm-hooks/useGenerateBorder.tsx:59](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/hooks/wasm-hooks/useGenerateBorder.tsx#L59)

Custom hook to generate images with borders using a Web Worker.

## Parameters

### options

`UseGenerateBordersProps`

Configuration options

## Returns

`object`

Object containing the generateBorders function

### generateBorders()

> **generateBorders**: (`files`, `option`, `param`) => `Promise`\<`void`\>

#### Parameters

##### files

`File`[]

##### option

[`BorderOptions`](../type-aliases/BorderOptions.md)

##### param

\{ `b`: `number`; `border_width`: `number`; `g`: `number`; `jpeg_quality`: `number`; `r`: `number`; \} | \{ `blur_sigma`: `number`; `brightness_adjustment`: `number`; `corner_radius`: `number`; `jpeg_quality`: `number`; \} | \{ `jpeg_quality`: `number`; `strength`: `number`; \}

#### Returns

`Promise`\<`void`\>
