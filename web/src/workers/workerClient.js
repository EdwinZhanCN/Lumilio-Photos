/**
 * This class is a wrapper around the Web Worker API to facilitate communication
 * with a WebAssembly worker.
 */
export class  WasmWorkerClient {
    /**
     * Creates an instance of WasmWorkerClient.
     */
    constructor() {
        this.generateThumbnailworker = new Worker(new URL('./thumbnail.worker.js', import.meta.url), {
            type: 'module'
        });
        this.hashAssetsworker = new Worker(new URL('./hash.worker.js', import.meta.url),{
            type:'module'
        })
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
     * Initializes the WebAssembly module in genThumbnail worker script.
     * @returns {Promise<void>}
     */
    async initGenThumbnailWASM(timeoutMs = 100000) {
        return new Promise((resolve, reject) => {
            const handler = (event) => {
                if (event.data.type === 'WASM_READY') {
                    clearTimeout(timeoutId);
                    this.generateThumbnailworker.removeEventListener('message', handler);
                    resolve({status: 'complete'});
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
            this.generateThumbnailworker.postMessage({type: 'INIT_WASM'});
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
            }

            // Add the event listener for the worker message
            this.generateThumbnailworker.addEventListener('message', handler)

            // Send the data to the worker for processing
            this.generateThumbnailworker.postMessage ({
                type: 'GENERATE_THUMBNAIL',
                data
            })

        })
    }

    /**
     * This function is used to initialize the WebAssembly module in genHash worker script.
     * @param {number} timeoutMs 
     * @returns 
     */
    async initGenHashWASM(timeoutMs = 100000) {
        return new Promise((resolve, reject) => {
            const handler = (event) => {
                if (event.data.type === 'WASM_READY') {
                    clearTimeout(timeoutId);
                    this.hashAssetsworker.removeEventListener('message', handler);
                    resolve({status: 'complete'});
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
            this.hashAssetsworker.postMessage({type: 'INIT_WASM'});
        });
    }


    /**
     * Processes files into hashcodes, sending the results hash back to the main thread.
     * You may want to use catch to handle the error.
     * @requires FileList
     * @param data {[FileList]} - The data to be processed. List of files.
     * @returns {Promise<any>}
     */
    async generateHash(data){
        return new Promise((resolve, reject) =>{
            /**
             * Handler for messages from the worker.
             * @param e {MessageEvent} - The message event from the worker.
             * @param e.data {string, Object} - The type of the message, and the data to be processed.
             * 
             */
            const handler = (e) => {
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
                            results: e.data.hashResult,
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
                        error.fileName = e.data.fileName;
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
            }
            // Add the event listener for the worker message
            this.hashAssetsworker.addEventListener('message', handler);
            // Send the data to the worker for processing
            this.hashAssetsworker.postMessage({
                type: 'GENERATE_HASH',
                data
            });
        })
    }



    /**
     * Terminates the genThumbnail worker.
     */
    terminateGenerateThumbnailWorker() {
        this.generateThumbnailworker.terminate();
    }

    /**
     * Terminates the genHash worker.
     */
    terminateGenerateHashWorker() {
        this.hashAssetsworker.terminate();
    }
}