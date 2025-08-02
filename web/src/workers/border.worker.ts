import init, {
  add_colored_border,
  create_frosted_border,
  add_vignette_border,
} from "@/wasm/border_wasm";

// --- 接口定义 (保持清晰) ---
interface ColoredBorderParam {
  border_width: number;
  r: number;
  g: number;
  b: number;
  jpeg_quality: number;
}

interface FrostedBorderParam {
  blur_sigma: number;
  brightness_adjustment: number;
  corner_radius: number;
  jpeg_quality: number;
}

interface VignetteBorderParam {
  strength: number;
  jpeg_quality: number;
}

interface WorkerMessage {
  type: "INIT_WASM" | "ABORT" | "GENERATE_BORDER";
  data?: {
    files: File[];
  };
  option?: "COLORED" | "FROSTED" | "VIGNETTE";
  param?: ColoredBorderParam | FrostedBorderParam | VignetteBorderParam;
}

// 结果将包含 UUID
interface ProcessedImageResult {
  uuid: string;
  originalFileName: string;
  borderedFileURL?: string;
  error?: string;
}

// --- 初始化流程控制 ---
// 使用 Promise 来控制初始化状态，这比布尔值更可靠
let initializationPromise: Promise<void> | null = null;

function initialize(): Promise<void> {
  if (initializationPromise) {
    return initializationPromise;
  }

  console.log("Initializing WebAssembly module...");
  initializationPromise = init()
    .then(() => {
      console.log("WebAssembly module initialized successfully.");
      self.postMessage({ type: "WASM_READY" });
    })
    .catch((error: any) => {
      const errorMessage =
        error.message ?? "Unknown worker initialization error";
      console.error("Error initializing WebAssembly module:", error);
      self.postMessage({ type: "ERROR", payload: { error: errorMessage } });
      throw new Error(errorMessage);
    });

  return initializationPromise;
}

let abortController = new AbortController();

self.onmessage = async (e: MessageEvent<WorkerMessage>) => {
  const { type, data, option, param } = e.data;

  // 根据消息类型分发任务
  switch (type) {
    case "INIT_WASM":
      initialize();
      break;

    case "ABORT":
      abortController.abort();
      abortController = new AbortController(); // 为下次任务重置
      break;

    case "GENERATE_BORDER":
      try {
        // 等待初始化完成，如果还未开始则会自动开始
        await initialize();

        if (!data || !option || !param) {
          throw new Error("Missing data, option, or param for GENERATE_BORDER");
        }

        const { files } = data;

        // Promise.all 会返回一个结果数组
        const resultsArray: ProcessedImageResult[] = await Promise.all(
          files.map(async (file) => {
            // **核心改动 #1: 为每个文件生成 UUID**
            const uuid = self.crypto.randomUUID();
            const originalFileName = file.name;

            if (abortController.signal.aborted) {
              return { uuid, originalFileName, error: "Operation aborted" };
            }

            try {
              const imageBuffer = await file.arrayBuffer();
              const imageData = new Uint8Array(imageBuffer);
              let borderedImageBytes: Uint8Array;

              // Wasm 函数调用逻辑保持不变
              switch (option) {
                case "COLORED": {
                  const p = param as ColoredBorderParam;
                  borderedImageBytes = add_colored_border(
                    imageData,
                    p.border_width,
                    p.r,
                    p.g,
                    p.b,
                    p.jpeg_quality,
                  );
                  break;
                }
                case "FROSTED": {
                  const p = param as FrostedBorderParam;
                  borderedImageBytes = create_frosted_border(
                    imageData,
                    p.blur_sigma,
                    p.brightness_adjustment,
                    p.corner_radius,
                    p.jpeg_quality,
                  );
                  break;
                }
                case "VIGNETTE": {
                  const p = param as VignetteBorderParam;
                  borderedImageBytes = add_vignette_border(
                    imageData,
                    p.strength,
                    p.jpeg_quality,
                  );
                  break;
                }
              }

              const blob = new Blob([borderedImageBytes]);
              const borderedFileURL = URL.createObjectURL(blob);

              return { uuid, originalFileName, borderedFileURL };
            } catch (error: any) {
              const errorMessage =
                typeof error === "string" ? error : error.message;
              return { uuid, originalFileName, error: errorMessage };
            }
          }),
        );

        // **核心改动 #2: 将结果数组转换为以 UUID 为键的对象**
        const resultsAsMap = resultsArray.reduce(
          (acc, result) => {
            acc[result.uuid] = {
              originalFileName: result.originalFileName,
              borderedFileURL: result.borderedFileURL,
              error: result.error,
            };
            return acc;
          },
          {} as { [uuid: string]: Omit<ProcessedImageResult, "uuid"> },
        );

        self.postMessage({
          type: "GENERATE_BORDER_COMPLETE",
          data: resultsAsMap,
        });
        abortController = new AbortController(); // 为下次任务重置
      } catch (initError: any) {
        // 这个 catch 主要用于捕获初始化失败的错误
        console.error(
          "Failed to process images due to initialization failure:",
          initError,
        );
        self.postMessage({
          type: "ERROR",
          payload: { error: initError.message },
        });
      }
      break;
  }
};
