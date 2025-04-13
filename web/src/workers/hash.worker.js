import init, { hash_asset } from '@/wasm/blake3_wasm.js'


let wasmReady = false;
let numberOfFilesProcessed = 0;
let abortController = new AbortController();


/**
 * Initializes the WebAssembly module in hashAssetsworker.
 * @returns {Promise<void>}
 */
async function initialize() {
    try {
        await init();
        wasmReady = true;
        self.postMessage({ type: 'WASM_READY' });
    } catch (error) {
        console.error('Error initializing genHash WebAssembly module:', error);
        self.postMessage(
            {
                type: 'ERROR', 
                payload:{
                    error: error.message || 'Unknown worker error',
                }
            }
        );
    }
}

self.onmessage = async (e) => {
    const { type, data } = e.data;
    switch (type) {
        case 'ABORT':
            abortController.abort();
            // clean
            hashResult.splice(0);
            break;
        case 'INIT_WASM':
            await initialize();
            break;
        case 'GENERATE_HASH':
            if (!wasmReady) {
                self.postMessage({ type: 'ERROR', error: 'WASM not initialized' });
                return;
            }
            try {
                const hashResult = await hashMultipleAssets(data);
                self.postMessage({
                    type: 'HASH_COMPLETE', 
                    hashResult: hashResult,
                });
            }catch(err){
                console.error(`Error generating hash for file ${i}:`, err);
                // Log detailed error information
                self.postMessage({
                    type: 'ERROR',
                    error: `[${i}]${err.message}`,
                    fileName: asset.name
                });
            }
            break;
    }
}

/**
 * This function hashes multiple assets.
 * @param {File[]} assets - The files (assets uploaded) to be hashed.
 * @returns {Promise<string[]>} - The hashes of the files. And the index are the same.
 */
async function hashMultipleAssets(assets) {
    // TODO: make this optional in system settings.
    const CONCURRENCY = assets[0]?.size > 100_000_000 ? 2 : 4; // dynamic concurrency level, consider to be changable.
    const hashResult = [];
    
    // process assets in batches
    for (let i = 0; i < assets.length; i += CONCURRENCY) {
        const batch = assets.slice(i, i + CONCURRENCY);
        const promises = batch.map(async (asset, batchIndex) => {
            const globalIndex = i + batchIndex;
            try {
                let arrayBuffer = await asset.arrayBuffer();
                const hash = await hash_asset(new Uint8Array(arrayBuffer));
                
                // release memory
                arrayBuffer = null; 
                
                return {
                    index: globalIndex,
                    hash
                };
            } catch(err) {
                console.error(`Error generating hash for [${globalIndex}]${asset.name}:`, err);
                return {
                    index: globalIndex,
                    hash: '0'.repeat(64), // 0-filled hash
                    error: err.message
                };
            } finally {
                // update progress counter
                self.postMessage({
                    type: 'PROGRESS',
                    payload: { processed: ++numberOfFilesProcessed }
                });
            }
        });

        // wait for all promises in this batch to resolve
        const batchResults = await Promise.all(promises);
        hashResult.push(...batchResults);
    }

    // The result is sorted by index.
    return hashResult.sort((a, b) => a.index - b.index).map(x => x.hash);
}