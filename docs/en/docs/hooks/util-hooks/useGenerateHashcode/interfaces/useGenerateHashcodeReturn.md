[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcodeReturn

# Interface: useGenerateHashcodeReturn

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:31](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/67c9aa6e9757c27514a72920fc1c7730f5f2ba78/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L31)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:38](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/67c9aa6e9757c27514a72920fc1c7730f5f2ba78/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L38)

#### Returns

`void`

***

### generateHashCodes()

> **generateHashCodes**: (`files`, `priority?`) => `Promise`\<`undefined` \| `HashcodeResult`[]\>

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:34](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/67c9aa6e9757c27514a72920fc1c7730f5f2ba78/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L34)

#### Parameters

##### files

`FileList` | `File`[]

##### priority?

[`ProcessingPriority`](../../../../utils/smartBatchSizing/enumerations/ProcessingPriority.md)

#### Returns

`Promise`\<`undefined` \| `HashcodeResult`[]\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:32](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/67c9aa6e9757c27514a72920fc1c7730f5f2ba78/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L32)

***

### progress

> **progress**: `null` \| [`HashcodeProgress`](HashcodeProgress.md)

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:33](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/67c9aa6e9757c27514a72920fc1c7730f5f2ba78/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L33)
