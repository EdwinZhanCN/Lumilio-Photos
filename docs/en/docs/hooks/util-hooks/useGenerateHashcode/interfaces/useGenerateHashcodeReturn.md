[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcodeReturn

# Interface: useGenerateHashcodeReturn

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:17](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L17)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:24](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L24)

#### Returns

`void`

***

### generateHashCodes()

> **generateHashCodes**: (`files`, `onChunkProcessed?`) => `Promise`\<`HashcodeResult`[] \| `undefined`\>

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:20](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L20)

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

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:18](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L18)

***

### progress

> **progress**: [`HashcodeProgress`](HashcodeProgress.md) \| `null`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:19](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L19)
