/**
 * This class is a wrapper around the Web Worker API to facilitate communication
 * with a WebAssembly worker.
 */
export class  WasmWorkerClient {
    /**
     * Creates an instance of WasmWorkerClient.
     * @param workerPath {string} - The path to the Web Worker script.
     */
    constructor( workerPath ) {
        this.worker = new Worker(new URL(workerPath, import.meta.url), {
            type: 'module',
        })
        console.log('Worker created', this.worker)
        this.eventTarget = new EventTarget();
    }

    /**
     * Adds a progress listener to the worker.
     * @param callback
     * @returns {function(): void}
     */
    addProgressListener(callback) {
        const handler = (e) => callback(e.detail);
        this.eventTarget.addEventListener('progress', handler);
        return () => this.eventTarget.removeEventListener('progress', handler);
    }

    /**
     * Initializes the WebAssembly module in current worker script.
     * @returns {Promise<void>}
     */
    async initWASM(timeoutMs = 10000) {
        return new Promise((resolve, reject) => {
            const handler = (event) => {
                if (event.data.type === 'WASM_READY') {
                    clearTimeout(timeoutId);
                    this.worker.removeEventListener('message', handler);
                    resolve({status: 'complete'});
                }
                if (event.data.type === 'ERROR') {
                    clearTimeout(timeoutId);
                    this.worker.removeEventListener('message', handler);
                    reject(new Error(event.data.payload?.error || 'WASM initialization failed'));
                }
            };

            const timeoutId = setTimeout(() => {
                this.worker.removeEventListener('message', handler);
                reject(new Error('WASM initialization timed out'));
            }, timeoutMs);

            this.worker.addEventListener('message', handler);
            this.worker.postMessage({type: 'INIT_WASM'});
        });
    }

    /**
     * Processes files in batches, sending the results thumbnails back to the main thread.
     * You may want to use catch to handle the error.
     * @requires FileList
     * @param data {[FileList,number,number]} - The data to be processed. List of files, batch index, and start index.
     * @returns {Promise<any>}
     */
    async generateThumbnail(data){
        return new Promise((resolve, reject) =>{
            /**
             * Handler for messages from the worker.
             * @param e {MessageEvent} - The message event from the worker.
             * @param e.data {string, Object} - The type of the message, and the data to be processed.
             */
            const handler = (e) => {
                // A listener for messages from the worker
                switch (e.data.type) {
                    case 'BATCH_COMPLETE':
                        console.log('BATCH_COMPLETE received', e.data.payload.results.length);
                        // In RESULT Message {data.}, the worker will send:
                        // - type: The type of the message
                        // - payload: The object containing the result
                        // - payload.batchIndex: The index of the batch
                        // - payload.results: The results of the batch, thumbnails specifically
                        resolve({
                            batchIndex: e.data.payload.batchIndex,
                            results: e.data.payload.results,
                            status: 'complete'
                        });
                        this.worker.removeEventListener('message', handler);
                        break;
                    case 'ERROR':
                        // In ERROR Message {data.}, the worker will send:
                        // - batchIndex: The index of the batch that caused the error
                        // - payload: The object containing the error details
                        // - payload.error: The error message
                        // - payload.errorName: The name of the error
                        // - payload.errorStack: The stack trace of the error
                        console.error(
                            `Worker error in batch ${e.data.batchIndex}: ${e.data.payload.errorName} - ${e.data.payload.error}`,
                            {
                                errorStack: e.data.payload.errorStack,
                                batchIndex: e.data.batchIndex,
                                errorDetails: e.data.payload,
                            }
                        );
                        const error = new Error(e.data.payload.error);
                        error.name = e.data.payload.errorName;
                        error.stack = e.data.payload.errorStack;
                        this.worker.removeEventListener('message', handler);
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
                    case 'WASM_READY':
                        resolve({status: 'in-complete'})
                        break;
                    default:
                        console.warn('Unknown message type:', e.data.type);
                }
            }

            // Add the event listener for the worker message
            this.worker.addEventListener('message', handler)

            // Send the data to the worker for processing
            this.worker.postMessage ({
                type: 'GENERATE_THUMBNAIL',
                data
            })

        })
    }

    /**
     * Terminates the worker.
     */
    terminate() {
        this.worker.terminate();
    }
}