import init, { generate_thumbnail } from '@/wasm/thumbnail_wasm.js';

/**
 * Interface describing the data passed with each worker message.
 */
interface WorkerMessageData {
    files?: File[];
    batchIndex?: number;
    startIndex?: number;
}

/**
 * Types of messages that can be sent to this worker.
 */
type WorkerMessageType = 'INIT_WASM' | 'GENERATE_THUMBNAIL';

/**
 * The total structure of a message event posted to this worker.
 */
interface WorkerMessageEvent {
    type: WorkerMessageType;
    data?: WorkerMessageData;
}

/**
 * The shape of the data we send back in success responses (thumbnails generated, etc.).
 */
interface BatchResult {
    index: number;
    url: string;
}

let wasmReady = false;
let numberOfFilesProcessed = 0;

/**
 * Initializes the WebAssembly module in thumbnail worker.
 */
async function initialize(): Promise<void> {
    try {
        await init();
        wasmReady = true;
        self.postMessage({ type: 'WASM_READY' });
    } catch (error) {
        console.error('Error initializing genThumbnail WebAssembly module:', error);
        self.postMessage({
            type: 'ERROR',
            payload: {
                error: (error as Error).message || 'Unknown worker error',
            },
        });
    }
}

/**
 * Handles incoming messages to this worker.
 */
self.onmessage = async (e: MessageEvent<WorkerMessageEvent>) => {
    const { type, data } = e.data;

    switch (type) {
        case 'INIT_WASM':
            await initialize();
            break;

        case 'GENERATE_THUMBNAIL': {
            if (!wasmReady) {
                self.postMessage({ type: 'ERROR', error: 'WASM not initialized' });
                return;
            }
            try {
                const { files = [], batchIndex = 0, startIndex = 0 } = data ?? {};
                const results = await generatePreview(files, batchIndex, startIndex);
                self.postMessage({
                    type: 'BATCH_COMPLETE',
                    payload: {
                        batchIndex,
                        results,
                    },
                });
            } catch (error) {
                self.postMessage({
                    type: 'ERROR',
                    payload: {
                        batchIndex: data?.batchIndex,
                        error: (error as Error).message || 'Unknown worker error',
                        errorName: (error as Error).name || 'Error',
                        errorStack: (error as Error).stack || '',
                    },
                });
            }
            break;
        }

        default:
            // Optionally handle unrecognized message types
            break;
    }
};

/**
 * The core function that generates the preview for each file.
 *
 * @param files - The files (images) to be processed.
 * @param batchIndex - The index of the batch being processed.
 * @param startIndex - The starting index of the files in the batch.
 * @returns Promise resolving to a list of batch results (thumbnail info).
 */
async function generatePreview(
    files: File[],
    batchIndex: number,
    startIndex: number
): Promise<BatchResult[]> {
    const results: BatchResult[] = [];

    for (let i = 0; i < files.length; i++) {
        const file = files[i];
        let url: string;

        // Check file type and generate appropriate preview
        if (file.type.startsWith('video/')) {
            // TODO: Implement video preview
            url = createVideoPreview();
        } else if (isRawFile(file)) {
            // TODO: Implement RAW file preview
            url = createRawPreview(file);
        } else {
            // For images, use WebAssembly to generate thumbnail
            try {
                const arrayBuffer = await file.arrayBuffer();
                const result = generate_thumbnail(new Uint8Array(arrayBuffer), 300);
                const blob = new Blob([result], { type: 'image/jpeg' });
                url = URL.createObjectURL(blob);
            } catch (err) {
                console.error('Error generating thumbnail:', err);
                // Log details
                console.warn('File that caused error:', {
                    name: file.name,
                    type: file.type,
                    size: file.size,
                });
                url = createDefaultPreview();
            }
        }

        results.push({
            index: startIndex + i,
            url,
        });

        self.postMessage({
            type: 'PROGRESS',
            payload: {
                batchIndex,
                processed: numberOfFilesProcessed,
            },
        });
        numberOfFilesProcessed++;
    }

    return results;
}

// Helper functions

/**
 * Checks if the provided file is a RAW image type.
 */
function isRawFile(file: File): boolean {
    const rawMimeTypes = [
        'image/x-canon-cr2',
        'image/x-nikon-nef',
        'image/x-sony-arw',
        'image/x-adobe-dng',
        'image/x-fuji-raf',
        'image/x-panasonic-rw2',
    ];
    const rawExtensions = ['cr2', 'nef', 'arw', 'raf', 'rw2', 'dng', 'cr3', '3fr', 'orf'];

    const extension = file.name.split('.').pop()?.toLowerCase();
    return (
        (extension && rawExtensions.includes(extension)) ||
        rawMimeTypes.includes(file.type)
    );
}

function createVideoPreview(): string {
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path fill="#ccc" d="M16 16c0 1.104-.896 2-2 2H4c-1.104 0-2-.896-2-2V8c0-1.104.896-2 2-2h10c1.104 0 2 .896 2 2v8zm4-10h-2v2h2v8h-2v2h4V6z"/>
</svg>`;
    return URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
}

function createRawPreview(file: File): string {
    const extension = file.name.split('.').pop()?.toUpperCase() || 'RAW';
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120" width="120" height="120">
<rect width="100%" height="100%" fill="#e2e8f0" rx="8" ry="8"/>
<text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle"
      font-family="system-ui, -apple-system, sans-serif"
      font-weight="600"
      fill="#475569">
  <tspan x="50%" dy="-0.6em" font-size="14">RAW</tspan>
  <tspan x="50%" dy="1.8em" font-size="12">${extension}</tspan>
</text>
</svg>`;
    return URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
}

function createDefaultPreview(): string {
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120" width="120" height="120">
<rect width="100%" height="100%" fill="#f1f5f9" rx="8" ry="8"/>
<text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle"
      font-family="system-ui, -apple-system, sans-serif"
      font-weight="600"
      fill="#64748b">
  <tspan x="50%" dy="0" font-size="12">Image</tspan>
  <tspan x="50%" dy="1.2em" font-size="10">Preview Failed</tspan>
</text>
</svg>`;
    return URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
}

// Ensures this file is treated as a module.
export {};