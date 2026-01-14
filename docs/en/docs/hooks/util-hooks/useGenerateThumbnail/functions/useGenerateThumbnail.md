[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateThumbnail](../index.md) / useGenerateThumbnail

# Function: useGenerateThumbnail()

> **useGenerateThumbnail**(): [`UseGenerateThumbnailReturn`](../interfaces/UseGenerateThumbnailReturn.md)

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:40](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/fc7235ea377566f4cc2f62016aec0acb0e31f77b/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L40)

Custom hook to generate thumbnails using a Web Worker.
It manages its own state and uses the shared worker client.
This hook must be used within a component tree wrapped by `<WorkerProvider />`.

## Returns

[`UseGenerateThumbnailReturn`](../interfaces/UseGenerateThumbnailReturn.md)

Hook state and actions for thumbnail generation.

## Author

Edwin Zhan

## Since

1.1.1
