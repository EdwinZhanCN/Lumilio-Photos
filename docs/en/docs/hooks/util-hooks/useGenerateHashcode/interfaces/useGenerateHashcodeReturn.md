[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcodeReturn

# Interface: useGenerateHashcodeReturn

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:31](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L31)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:38](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L38)

#### Returns

`void`

***

### generateHashCodes()

> **generateHashCodes**: (`files`, `priority?`) => `Promise`\<`undefined` \| `HashcodeResult`[]\>

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:34](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L34)

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

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:32](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L32)

***

### progress

> **progress**: `null` \| [`HashcodeProgress`](HashcodeProgress.md)

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:33](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L33)
