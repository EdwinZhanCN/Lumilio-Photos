[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcodeReturn

# Interface: useGenerateHashcodeReturn

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:31](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7aef6035d4ec85d126436b63a0810fa5cfd946b/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L31)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:38](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7aef6035d4ec85d126436b63a0810fa5cfd946b/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L38)

#### Returns

`void`

***

### generateHashCodes()

> **generateHashCodes**: (`files`, `priority?`) => `Promise`\<`undefined` \| `HashcodeResult`[]\>

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:34](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7aef6035d4ec85d126436b63a0810fa5cfd946b/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L34)

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

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:32](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7aef6035d4ec85d126436b63a0810fa5cfd946b/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L32)

***

### progress

> **progress**: `null` \| [`HashcodeProgress`](HashcodeProgress.md)

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:33](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7aef6035d4ec85d126436b63a0810fa5cfd946b/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L33)
