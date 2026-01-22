import createExiv2Module, { type Exiv2Module } from "@/wasm/exiv2/exiv2";
import wasmUrl from "@/wasm/exiv2/exiv2.wasm?url";
let abortController = new AbortController();
let exiv2ModulePromise: Promise<Exiv2Module> | null = null;

const getExiv2Module = () => {
  if (!exiv2ModulePromise) {
    exiv2ModulePromise = createExiv2Module({
      // 3. 这里的 path 参数无视掉，直接返回 Vite 给我们的正确 URL
      locateFile: () => wasmUrl,
    });
  }
  return exiv2ModulePromise;
};

const normalizeExif = (data: any): Record<string, any> => {
  if (!data || typeof data !== "object") return {};
  const { exif = {}, iptc = {}, xmp = {} } = data as Record<string, any>;
  return { ...exif, ...iptc, ...xmp };
};

interface WorkerMessage {
  type: "ABORT" | "EXTRACT_EXIF";
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
    case "ABORT":
      abortController.abort();
      abortController = new AbortController();
      break;

    case "EXTRACT_EXIF": {
      if (!data || !data.files || !Array.isArray(data.files)) {
        self.postMessage({ type: "ERROR", payload: { error: "Invalid data" } });
        return;
      }

      const { files } = data;

      try {
        const exiv2 = await getExiv2Module();
        const results: WorkerExifResult[] = [];
        let aborted = false;

        for (let i = 0; i < files.length; i++) {
          if (abortController.signal.aborted) {
            aborted = true;
            break;
          }

          const file = files[i];

          try {
            const buffer = await file.arrayBuffer();
            if (abortController.signal.aborted) {
              aborted = true;
              break;
            }

            const exifData = normalizeExif(exiv2.read(new Uint8Array(buffer)));
            results.push({ index: i, exifData });
          } catch (err) {
            const errMsg =
              (err as Error).message || "Failed to extract metadata";
            results.push({ index: i, exifData: {}, error: errMsg });
          }

          self.postMessage({
            type: "PROGRESS",
            payload: {
              processed: i + 1,
              total: files.length,
            },
          });
        }

        self.postMessage({
          type: "EXIF_COMPLETE",
          payload: { results, aborted },
        });
      } catch (error) {
        const errMsg = (error as Error).message || "Unknown worker error";
        self.postMessage({ type: "ERROR", payload: { error: errMsg } });
      } finally {
        abortController = new AbortController();
      }
      break;
    }

    default:
      self.postMessage({
        type: "ERROR",
        payload: { error: "Unknown message type" },
      });
  }
};
