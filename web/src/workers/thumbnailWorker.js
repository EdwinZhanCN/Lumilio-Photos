// web/src/workers/thumbnailWorker.js
import  {useWasm} from "@/hooks/useWasm.jsx";

const {wasmReady, generate_thumbnail } = useWasm()

self.onmessage = async (event) => {
    const { type, payload } = event.data;

    if (type === 'PROCESS_FILES') {
        const { files, batchIndex, startIndex } = payload;

        try {
            const results = [];

            for (let i = 0; i < files.length; i++) {
                const file = files[i];
                let url;

                // Generate appropriate preview based on file type
                if (file.type.startsWith('video/')) {
                    url = createVideoPreview();
                } else if (isRawFile(file)) {
                    url = createRawPreview(file);
                } else {
                    // For images, use WASM to generate thumbnail
                    try {
                        const arrayBuffer = await file.arrayBuffer();
                        const result = generate_thumbnail(new Uint8Array(arrayBuffer), 300);
                        const blob = new Blob([result], { type: 'image/jpeg' });
                        url = URL.createObjectURL(blob);
                    } catch (err) {
                        console.error('Error generating thumbnail:', err);
                        // Fallback to default image if WASM processing fails
                        url = createDefaultPreview(file);
                    }
                }

                results.push({
                    index: startIndex + i,
                    url
                });

                // Report progress periodically
                if (i % 2 === 0 || i === files.length - 1) {
                    self.postMessage({
                        type: 'PROGRESS',
                        payload: {
                            batchIndex,
                            processed: i + 1,
                            total: files.length
                        }
                    });
                }
            }

            self.postMessage({
                type: 'BATCH_COMPLETE',
                payload: {
                    batchIndex,
                    results
                }
            });
        } catch (error) {
            self.postMessage({
                type: 'ERROR',
                payload: {
                    batchIndex,
                    error: error.message
                }
            });
        }
    }
};

// Helper functions
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