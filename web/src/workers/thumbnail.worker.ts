import init, { generate_thumbnail } from "../wasm/thumbnail/thumbnail_wasm";

interface WorkerMessageData {
  files?: File[];
  batchIndex?: number;
  startIndex?: number;
}

type WorkerMessageType = "ABORT" | "GENERATE_THUMBNAIL";

interface WorkerMessageEvent {
  type: WorkerMessageType;
  data?: WorkerMessageData;
}

interface BatchResult {
  index: number;
  url: string;
}

// --- Initialization Control ---
let initializationPromise: Promise<void> | null = null;

function initialize(): Promise<void> {
  if (initializationPromise) {
    return initializationPromise;
  }

  initializationPromise = new Promise((resolve, reject) => {
    init()
      .then(() => {
        self.postMessage({ type: "WASM_READY" });
        resolve();
      })
      .catch((error) => {
        const errorMessage =
          (error as Error).message || "Unknown worker initialization error";
        console.error(
          "Error initializing genThumbnail WebAssembly module:",
          error,
        );
        self.postMessage({ type: "ERROR", payload: { error: errorMessage } });
        reject(new Error(errorMessage));
      });
  });

  return initializationPromise;
}

// --- Abort Control ---
let abortController = new AbortController();

// --- Main Logic ---
self.onmessage = async (e: MessageEvent<WorkerMessageEvent>) => {
  const { type, data } = e.data;

  switch (type) {
    case "ABORT":
      abortController.abort();
      abortController = new AbortController(); // Reset for next task
      break;

    case "GENERATE_THUMBNAIL": {
      abortController = new AbortController(); // New controller for this job
      let numberOfFilesProcessed = 0;

      try {
        await initialize();

        const { files = [], batchIndex = 0, startIndex = 0 } = data ?? {};

        const results = await generatePreview(
          files,
          startIndex,
          // eslint-disable-next-line @typescript-eslint/no-unused-vars
          (_processedInBatch) => {
            // Progress callback
            numberOfFilesProcessed++;
            self.postMessage({
              type: "PROGRESS",
              payload: {
                batchIndex,
                processed: numberOfFilesProcessed, // This is total processed in this task
                total: files.length,
              },
            });
          },
        );

        self.postMessage({
          type: "BATCH_COMPLETE",
          payload: {
            batchIndex,
            results,
          },
        });
      } catch (error) {
        self.postMessage({
          type: "ERROR",
          payload: {
            batchIndex: data?.batchIndex,
            error: (error as Error).message || "Unknown worker error",
            errorName: (error as Error).name || "Error",
            errorStack: (error as Error).stack || "",
          },
        });
      }
      break;
    }

    default:
      // Handle unrecognized message types
      break;
  }
};

async function generatePreview(
  files: File[],
  startIndex: number,
  onProgress: (processedInBatch: number) => void,
): Promise<BatchResult[]> {
  const results: BatchResult[] = [];
  const signal = abortController.signal;

  for (let i = 0; i < files.length; i++) {
    if (signal.aborted) {
      console.log("Thumbnail generation aborted.");
      break;
    }

    const file = files[i];
    let url: string;

    if (file.type.startsWith("video/")) {
      url = createVideoPreview();
    } else if (isRawFile(file)) {
      // Do not generate thumbnails for RAW files as requested
      url = ""; 
    } else {
      try {
        const arrayBuffer = await file.arrayBuffer();
        if (signal.aborted) break;
        const result = generate_thumbnail(new Uint8Array(arrayBuffer), 300);
        const blob = new Blob([result.buffer as ArrayBuffer], {
          type: "image/jpeg",
        });
        url = URL.createObjectURL(blob);
      } catch (err) {
        console.error("Error generating thumbnail:", err);
        url = createDefaultPreview();
      }
    }

    results.push({
      index: startIndex + i,
      url,
    });

    onProgress(i + 1);
  }

  return results;
}

// --- Helper Functions ---
function isRawFile(file: File): boolean {
  const rawMimeTypes = [
    "image/x-canon-cr2",
    "image/x-nikon-nef",
    "image/x-sony-arw",
    "image/x-adobe-dng",
    "image/x-fuji-raf",
    "image/x-panasonic-rw2",
  ];
  const rawExtensions = [
    "cr2",
    "nef",
    "arw",
    "raf",
    "rw2",
    "dng",
    "cr3",
    "3fr",
    "orf",
  ];

  const extension = file.name.split(".").pop()?.toLowerCase();
  return (
    (extension && rawExtensions.includes(extension)) ||
    rawMimeTypes.includes(file.type)
  );
}

function createVideoPreview(): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path fill="#ccc" d="M16 16c0 1.104-.896 2-2 2H4c-1.104 0-2-.896-2-2V8c0-1.104.896-2 2-2h10c1.104 0 2 .896 2 2v8zm4-10h-2v2h2v8h-2v2h4V6z"/>
</svg>`;
  return URL.createObjectURL(new Blob([svg], { type: "image/svg+xml" }));
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
  return URL.createObjectURL(new Blob([svg], { type: "image/svg+xml" }));
}

export {};
