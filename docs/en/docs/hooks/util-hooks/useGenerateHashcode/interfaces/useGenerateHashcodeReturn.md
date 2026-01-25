[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcodeReturn

# Interface: useGenerateHashcodeReturn

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:17](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L17)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:24](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L24)

#### Returns

`void`

***

### generateHashCodes()

> **generateHashCodes**: (`files`, `onChunkProcessed?`) => `Promise`\<`HashcodeResult`[] \| `undefined`\>

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:20](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L20)

#### Parameters

##### files

`FileList` | `File`[]

##### onChunkProcessed?

(`result`) => `void`

#### Returns

`Promise`\<`HashcodeResult`[] \| `undefined`\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:18](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L18)

***

### progress

> **progress**: [`HashcodeProgress`](HashcodeProgress.md) \| `null`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:19](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L19)
