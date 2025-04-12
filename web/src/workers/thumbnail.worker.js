import init, { generate_thumbnail } from '@/wasm/thumbnail_wasm.js';

let wasmReady = false;
let numberOfFilesProcessed = 0;

/**
 * Initializes the WebAssembly module in thumbnail worker.
 * @returns {Promise<void>}
 */
async function initialize() {
    await init();
    wasmReady = true;
    self.postMessage({ type: 'WASM_READY' });
}


self.onmessage = async (e) => {
    const { type, data } = e.data;
    switch (type){
        case 'INIT_WASM':
            await initialize();
            break;
        case 'GENERATE_THUMBNAIL':
            if(!wasmReady){
                self.postMessage({ type: 'ERROR', error: 'WASM not initialized' });
                return;
            }
            try {
                const {files, batchIndex, startIndex} = data;
                const results = await generatePreview(files, batchIndex, startIndex);
                self.postMessage({
                    type: 'BATCH_COMPLETE',
                    payload: {
                        batchIndex,
                        results
                    }
                });
            }catch (error) {
                self.postMessage({
                    type: 'ERROR',
                    payload: {
                        batchIndex: data.batchIndex,
                        error: error.message || 'Unknown worker error',
                        errorName: error.name || 'Error',
                        errorStack: error.stack || ''
                    }
                });
            }
    }
}

/**
 * The core function that generates the preview for each file.
 * @param {File[]} files - The files (images uploaded) to be processed.
 * @param {number} batchIndex - The index of the batch being processed.
 * @param {number} startIndex - The starting index we want of the files in the batch.
 * @returns {Promise<Array>} - Contains the results of the batch, including the generated thumbnails.
 */
async function generatePreview(files, batchIndex, startIndex) {
    // Prepare the results array
    const results = [];

    // Iterate through the files and generate previews
    for (let i = 0; i < files.length; i++) {
        const file = files[i];
        // The indicator path for the file in the batch
        let url;

        // Check the file type and generate the appropriate preview
        if (file.type.startsWith('video/')) {
            // TODO: 处理视频文件预览生成
            url = createVideoPreview();
        } else if (isRawFile(file)) {
            // TODO: 处理RAW文件预览生成
            url = createRawPreview(file);
        } else {
            // For IMAGES ONLY, use WebAssembly to generate the thumbnail
            try {
                const arrayBuffer = await file.arrayBuffer();
                const result = generate_thumbnail(new Uint8Array(arrayBuffer), 300);
                const blob = new Blob([result], { type: 'image/jpeg' });
                url = URL.createObjectURL(blob);
            } catch (err) {
                console.error('Error generating thumbnail:', err);
                // Log detailed error information
                console.warn('File that caused error:', {
                    name: file.name,
                    type: file.type,
                    size: file.size
                });
                // if thumbnail generation fails, create a default preview
                url = createDefaultPreview(file);
            }
        }

        // Add the result to the results array
        results.push({
            index: startIndex + i,
            url
        });


        self.postMessage({
            type: 'PROGRESS',
            payload: {
                batchIndex,
                processed: numberOfFilesProcessed,
            }
        });
        numberOfFilesProcessed++;
    }

    return results;
}

// Helper functions that creates placeholder images for invalid or unsupported files
function isRawFile(file) {
    const rawMimeTypes = [
        'image/x-canon-cr2',
        'image/x-nikon-nef',
        'image/x-sony-arw',
        'image/x-adobe-dng',
        'image/x-fuji-raf',
        'image/x-panasonic-rw2'
    ];

    const rawExtensions = ['cr2', 'nef', 'arw', 'raf', 'rw2', 'dng', 'cr3', '3fr', 'orf'];

    return rawMimeTypes.includes(file.type) ||
        rawExtensions.includes(file.name.split('.').pop().toLowerCase());
}

//// SVG Placeholder Functions
function createVideoPreview() {
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path fill="#ccc" d="M16 16c0 1.104-.896 2-2 2H4c-1.104 0-2-.896-2-2V8c0-1.104.896-2 2-2h10c1.104 0 2 .896 2 2v8zm4-10h-2v2h2v8h-2v2h4V6z"/>
</svg>`;
    return URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }));
}

function createRawPreview(file) {
    const extension = file.name.split('.').pop().toUpperCase();
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

function createDefaultPreview(file) {
    // Creates a placeholder for images that failed to process
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



