import { MODULE } from "@immich/justified-layout-wasm/pkg/justified-layout-wasm-module.js";

export interface JustifiedLayoutOptions {
  rowHeight: number;
  rowWidth: number;
  spacing: number;
  heightTolerance: number;
}

interface JustifiedWasmExports extends WebAssembly.Exports {
  memory: WebAssembly.Memory;
  get_justified_layout(
    aspectRatios: number,
    aspectRatiosLength: number,
    rowHeight: number,
    rowWidth: number,
    spacing: number,
    heightTolerance: number,
  ): [number, number];
  __wbindgen_export_0: WebAssembly.Table;
  __wbindgen_malloc(size: number, alignment: number): number;
  __wbindgen_free(pointer: number, size: number, alignment: number): void;
  __wbindgen_start(): void;
}

let wasm: JustifiedWasmExports | undefined;
let float32Memory: Float32Array | undefined;

function ensureInitialized() {
  if (wasm) return wasm;

  let exports: JustifiedWasmExports | undefined;
  const imports = {
    wbg: {
      __wbindgen_init_externref_table() {
        if (!exports) throw new Error("justified layout WASM is not initialized");
        const table = exports.__wbindgen_export_0;
        const offset = table.grow(4);
        table.set(0, undefined);
        table.set(offset, undefined);
        table.set(offset + 1, null);
        table.set(offset + 2, true);
        table.set(offset + 3, false);
      },
    },
  };

  const module = new WebAssembly.Module(MODULE as unknown as BufferSource);
  const instance = new WebAssembly.Instance(module, imports);
  exports = instance.exports as JustifiedWasmExports;
  wasm = exports;
  float32Memory = undefined;
  wasm.__wbindgen_start();
  return wasm;
}

/** Compile and validate the embedded module before a worker reports ready. */
export function initializeJustifiedLayout() {
  ensureInitialized();
}

function getFloat32Memory(exports: JustifiedWasmExports) {
  if (!float32Memory || float32Memory.byteLength === 0) {
    float32Memory = new Float32Array(exports.memory.buffer);
  }
  return float32Memory;
}

function getJustifiedLayout(aspectRatios: Float32Array, options: JustifiedLayoutOptions) {
  const exports = ensureInitialized();
  const pointer = exports.__wbindgen_malloc(aspectRatios.length * 4, 4) >>> 0;
  getFloat32Memory(exports).set(aspectRatios, pointer / 4);

  const [resultPointer, resultLength] = exports.get_justified_layout(
    pointer,
    aspectRatios.length,
    options.rowHeight,
    options.rowWidth,
    options.spacing,
    options.heightTolerance,
  );
  const result = getFloat32Memory(exports)
    .subarray(resultPointer / 4, resultPointer / 4 + resultLength)
    .slice();
  exports.__wbindgen_free(resultPointer, resultLength * 4, 4);
  return result;
}

/** Thin typed view over the package's flat Float32Array result. */
export class JustifiedLayout {
  private readonly layout: Float32Array;

  constructor(aspectRatios: Float32Array, options: JustifiedLayoutOptions) {
    this.layout =
      aspectRatios.length === 0 ? new Float32Array(4) : getJustifiedLayout(aspectRatios, options);
  }

  get containerWidth() {
    return this.layout[0];
  }

  get containerHeight() {
    return this.layout[1];
  }

  getPosition(index: number) {
    const offset = index * 4 + 4;
    return {
      top: this.layout[offset],
      left: this.layout[offset + 1],
      width: this.layout[offset + 2],
      height: this.layout[offset + 3],
    };
  }
}
