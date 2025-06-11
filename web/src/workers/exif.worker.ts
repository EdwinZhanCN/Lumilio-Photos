import { parseMetadata } from '@uswriting/exiftool';

let abortController = new AbortController();

interface WorkerMessage {
    type: 'ABORT' | 'EXTRACT_EXIF';
    data?: {
        files: File[];
    };
}

interface WorkerExifResult {
    index: number;
    exifData: Record<string, any>;
    error?: string;
}

self.onmessage = async (e: MessageEvent<WorkerMessage>) => {
    const { type, data } = e.data;

    switch (type) {
        case 'ABORT':
            abortController.abort();
            break;

        case 'EXTRACT_EXIF': {
            if (!data || !data.files || !Array.isArray(data.files)) {
                self.postMessage({ type: 'ERROR', payload: { error: 'Invalid data' } });
                return;
            }

            const { files } = data;

            try {
                const results: WorkerExifResult[] = [];
                for (let i = 0; i < files.length; i++) {
                    if (abortController.signal.aborted) {
                        break;
                    }
                    const file = files[i];
                    const exifData = await parseMetadata(file, {
                        args: ["-json"],
                        transform: (data) => JSON.parse(data)
                    });
                    results.push({ index: i, exifData });
                    // Send progress update
                    self.postMessage({
                        type: 'PROGRESS',
                        payload: {
                            processed: i + 1,
                            total: files.length
                        }
                    });
                }
                self.postMessage({ type: 'EXIF_COMPLETE', payload: { results } });
            } catch (error) {
                const errMsg = (error as Error).message || 'Unknown worker error';
                self.postMessage({ type: 'ERROR', payload: { error: errMsg } });
            }
            break;
        }

        default:
            self.postMessage({ type: 'ERROR', payload: { error: 'Unknown message type' } });
    }
}
