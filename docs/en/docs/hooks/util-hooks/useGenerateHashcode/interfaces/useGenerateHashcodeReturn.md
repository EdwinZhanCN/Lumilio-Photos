[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcodeReturn

# Interface: useGenerateHashcodeReturn

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:31](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/7a82081bf9fc00044785b3bff4727efe730d6ec5/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L31)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:38](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/7a82081bf9fc00044785b3bff4727efe730d6ec5/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L38)

#### Returns

`void`

***

### generateHashCodes()

> **generateHashCodes**: (`files`, `priority?`) => `Promise`\<`HashcodeResult`[] \| `undefined`\>

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:34](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/7a82081bf9fc00044785b3bff4727efe730d6ec5/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L34)

#### Parameters

##### files

`FileList` | `File`[]

##### priority?

[`ProcessingPriority`](../../../../utils/smartBatchSizing/enumerations/ProcessingPriority.md)

#### Returns

`Promise`\<`HashcodeResult`[] \| `undefined`\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:32](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/7a82081bf9fc00044785b3bff4727efe730d6ec5/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L32)

***

### progress

> **progress**: [`HashcodeProgress`](HashcodeProgress.md) \| `null`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:33](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/7a82081bf9fc00044785b3bff4727efe730d6ec5/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L33)
