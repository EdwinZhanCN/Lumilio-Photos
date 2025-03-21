import { useState, useEffect } from 'react';
import init, { generate_thumbnail } from '@/wasm/thumbnail_wasm';

export function useWasm() {
    const [wasmReady, setWasmReady] = useState(false);

    useEffect(() => {
        const loadWasm = async () => {
            await init();
            setWasmReady(true);
        };
        loadWasm();
    }, []);

    return {
        wasmReady,
        generateThumbnail: async (file: File, maxSize: number) => {
            const buffer = await file.arrayBuffer();
            const result = generate_thumbnail(
                new Uint8Array(buffer),
                maxSize
            );
            return new Blob([result.data], { type: 'image/jpeg' });
        }
    };
}