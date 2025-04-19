/**
 * This class is a wrapper around the Web Worker API to facilitate communication
 * with a WebAssembly worker.
 */
export class WasmWorkerClient {
    private generateThumbnailworker: Worker;
    private hashAssetsworker: Worker;
    private eventTarget: EventTarget;

    /**
     * Creates an instance of WasmWorkerClient.
     */
    constructor() {
        this.generateThumbnailworker = new Worker(new URL('./thumbnail.worker.ts', import.meta.url), {
            type: 'module'
        });
        this.hashAssetsworker = new Worker(new URL('./hash.worker.ts', import.meta.url), {
            type: 'module'
        });
        this.eventTarget = new EventTarget();
    }

    /**
     * Adds a progress listener to the worker.
     * @param callback - Function to handle progress events
     * @returns {function(): void} - Function to remove the event listener
     */
    addProgressListener(callback: (detail: any) => void): () => void {
        const handler = (e: CustomEvent) => callback(e.detail);
        this.eventTarget.addEventListener('progress', handler as EventListener);
        return () => this.eventTarget.removeEventListener('progress', handler as EventListener);
    }

    /**
     * Initializes the WebAssembly module in genThumbnail worker script.
     * @param timeoutMs - Timeout in milliseconds
     * @returns {Promise<{status: string}>}
     */
    async initGenThumbnailWASM(timeoutMs: number = 100000): Promise<{ status: string }> {
        return new Promise((resolve, reject) => {
            const handler = (event: MessageEvent) => {
                if (event.data.type === 'WASM_READY') {
                    clearTimeout(timeoutId);
                    this.generateThumbnailworker.removeEventListener('message', handler);
                    resolve({ status: 'complete' });
                }
                if (event.data.type === 'ERROR') {
                    clearTimeout(timeoutId);
                    this.generateThumbnailworker.removeEventListener('message', handler);
                    reject(new Error(event.data.payload?.error || 'WASM initialization failed'));
                }
            };

            const timeoutId = setTimeout(() => {
                this.generateThumbnailworker.removeEventListener('message', handler);
                reject(new Error('WASM initialization timed out'));
            }, timeoutMs);

            this.generateThumbnailworker.addEventListener('message', handler);
            this.generateThumbnailworker.postMessage({ type: 'INIT_WASM' });
        });
    }

    /**
     * Processes files in batches, sending the results thumbnails back to the main thread.
     * You may want to use catch to handle the error.-
     * @param data - The data to be processed. List of files, batch index, and start index.
     * @returns {Promise<{batchIndex: number, results: any[], status: string}>}
     */
    async generateThumbnail(data: { files: FileList|File[], batchIndex: number, startIndex: number }): Promise<{ batchIndex: number, results: any[], status: string }> {
        return new Promise((resolve, reject) => {
            /**
             * Handler for messages from the worker.
             * @param e {MessageEvent} - The message event from the worker.
             */
            const handler = (e: MessageEvent) => {
                // A listener for messages from the worker
                switch (e.data.type) {
                    case 'BATCH_COMPLETE':
                        console.log('BATCH_COMPLETE received', e.data.payload.results.length);
                        // In BATCH_COMPLETE Message {data.}, the worker will send:
                        // - type: The type of the message
                        // - payload: The object containing the result
                        // - payload.batchIndex: The index of the batch
                        // - payload.results: The results of the batch, thumbnails specifically
                        resolve({
                            batchIndex: e.data.payload.batchIndex,
                            results: e.data.payload.results,
                            status: 'complete'
                        });
                        this.generateThumbnailworker.removeEventListener('message', handler);
                        break;
                    case 'ERROR':
                        // In ERROR Message {data.}, the worker will send:
                        // - batchIndex: The index of the batch that caused the error
                        // - payload: The object containing the error details
                        // - payload.error: The error message
                        // - payload.errorName: The name of the error
                        // - payload.errorStack: The stack trace of the error
                        console.error(
                            `GenerateThumbnail worker error in batch ${e.data.batchIndex}: ${e.data.payload.errorName} - ${e.data.payload.error}`,
                            {
                                errorStack: e.data.payload.errorStack,
                                batchIndex: e.data.batchIndex,
                                errorDetails: e.data.payload,
                            }
                        );
                        const error = new Error(e.data.payload.error);
                        error.name = e.data.payload.errorName;
                        error.stack = e.data.payload.errorStack;
                        this.generateThumbnailworker.removeEventListener('message', handler);
                        // use catch to handle the error
                        reject(error);
                        break;
                    case 'PROGRESS':
                        // For Frontend progress bar
                        // the numberProcessed is the number of files processed out of the total files user uploaded
                        this.eventTarget.dispatchEvent(new CustomEvent('progress', {
                            detail: {
                                batchIndex: e.data.payload.batchIndex,
                                processed: e.data.payload.processed
                            }
                        }));
                        break;
                    default:
                        console.warn('Unknown message type from genThumbnailWorker:', e.data.type);
                }
            };

            // Add the event listener for the worker message
            this.generateThumbnailworker.addEventListener('message', handler);

            // Send the data to the worker for processing
            this.generateThumbnailworker.postMessage({
                type: 'GENERATE_THUMBNAIL',
                data
            });
        });
    }

    /**
     * This function is used to initialize the WebAssembly module in genHash worker script.
     * @param timeoutMs - Timeout in milliseconds
     * @returns {Promise<{status: string}>}
     */
    async initGenHashWASM(timeoutMs: number = 100000): Promise<{ status: string }> {
        return new Promise((resolve, reject) => {
            const handler = (event: MessageEvent) => {
                if (event.data.type === 'WASM_READY') {
                    clearTimeout(timeoutId);
                    this.hashAssetsworker.removeEventListener('message', handler);
                    resolve({ status: 'complete' });
                }
                if (event.data.type === 'ERROR') {
                    clearTimeout(timeoutId);
                    this.hashAssetsworker.removeEventListener('message', handler);
                    reject(new Error(event.data.payload?.error || 'WASM initialization failed'));
                }
            };

            const timeoutId = setTimeout(() => {
                this.hashAssetsworker.removeEventListener('message', handler);
                reject(new Error('WASM initialization timed out'));
            }, timeoutMs);

            this.hashAssetsworker.addEventListener('message', handler);
            this.hashAssetsworker.postMessage({ type: 'INIT_WASM' });
        });
    }

    /**
     * Processes files into hashcodes, sending the results hash back to the main thread.
     * You may want to use catch to handle the error.
     * @requires FileList
     * @param data - The data to be processed. List of files.
     * @returns {Promise<{hashResults: Array<{index: number, hash: string}>, status: string}>}
     */
    async generateHash(data: FileList): Promise<{ hashResults: Array<{ index: number, hash: string }>, status: string }> {
        return new Promise((resolve, reject) => {
            /**
             * Handler for messages from the worker.
             * @param e {MessageEvent} - The message event from the worker.
             */
            const handler = (e: MessageEvent) => {
                // A listener for messages from the worker
                switch (e.data.type) {
                    case 'HASH_COMPLETE':
                        // In HASH_COMPLETE Message {data.}, the worker will send:
                        // - type: The type of the message
                        // - hashResult: The list contains the all hashcode results
                        // [
                        //     {index: 0, hash: 'f0e1...'},
                        //     {index: 1, hash: '0000...'},
                        //     {index: 2, hash: 'a1b2...'},
                        //     ...
                        // ]
                        resolve({
                            // The hashResult is the list contains the all hashcode results
                            // The hash result if an array of objects, [{index:number,hash:string},...]
                            hashResults: e.data.hashResult,
                            status: 'complete'
                        });
                        this.hashAssetsworker.removeEventListener('message', handler);
                        break;
                    case 'ERROR':
                        // In ERROR Message {data.}, the worker will send:
                        // - type: The type of the message
                        // - error: The error message [index]err.message
                        // - fileName: The name of the file that caused the error
                        console.error(
                            `Hash worker error: ${e.data.error}`,
                            {
                                fileName: e.data.fileName,
                                errorDetails: e.data.error
                            }
                        );
                        const error = new Error(e.data.error);
                        (error as any).fileName = e.data.fileName;
                        this.hashAssetsworker.removeEventListener('message', handler);
                        reject(error);
                        break;
                    case 'PROGRESS':
                        // For Frontend progress bar
                        // the numberProcessed is the number of files processed out of the total files user uploaded
                        this.eventTarget.dispatchEvent(new CustomEvent('progress', {
                            detail: {
                                processed: e.data.payload.processed
                            }
                        }));
                        break;
                    default:
                        console.warn('Unknown message type from genHashWorker:', e.data.type);
                }
            };
            // Add the event listener for the worker message
            this.hashAssetsworker.addEventListener('message', handler);
            // Send the data to the worker for processing
            this.hashAssetsworker.postMessage({
                type: 'GENERATE_HASH',
                data
            });
        });
    }

    /**
     * Terminates the genThumbnail worker.
     */
    terminateGenerateThumbnailWorker(): void {
        this.generateThumbnailworker.terminate();
    }

    /**
     * Terminates the genHash worker.
     */
    terminateGenerateHashWorker(): void {
        this.hashAssetsworker.terminate();
    }
}