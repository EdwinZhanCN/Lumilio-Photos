[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcodeReturn

# Interface: useGenerateHashcodeReturn

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/1644752835268dce152ae5a6ed8e77af6920f217/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L26)

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:32](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/1644752835268dce152ae5a6ed8e77af6920f217/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L32)

#### Returns

`void`

***

### generateHashCodes()

> **generateHashCodes**: (`files`) => `Promise`\<`undefined` \| `HashcodeResult`[]\>

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:29](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/1644752835268dce152ae5a6ed8e77af6920f217/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L29)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<`undefined` \| `HashcodeResult`[]\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:27](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/1644752835268dce152ae5a6ed8e77af6920f217/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L27)

***

### progress

> **progress**: `null` \| [`HashcodeProgress`](HashcodeProgress.md)

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/1644752835268dce152ae5a6ed8e77af6920f217/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L28)
