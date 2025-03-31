// hooks/useWasm.jsx
import { useState, useEffect, useRef } from 'react';
import init, { generate_thumbnail } from '@/wasm/thumbnail_wasm';

export function useWasm() {
    const [wasmReady, setWasmReady] = useState(false);
    const initPromise = useRef(null);

    useEffect(() => {
        // Create a singleton initialization promise
        if (!initPromise.current) {
            initPromise.current = init()
                .then(() => {
                    console.log('WASM module initialized successfully');
                    setWasmReady(true);
                })
                .catch(err => {
                    console.error('Failed to initialize WASM module:', err);
                });
        }
    }, []);

    const generateThumbnail = async (file, maxSize) => {
        // Ensure WASM is initialized before proceeding
        if (!wasmReady) {
            await initPromise.current;
        }

        const arrayBuffer = await file.arrayBuffer();
        const result = generate_thumbnail(new Uint8Array(arrayBuffer), maxSize);
        return new Blob([result], { type: 'image/jpeg' });
    };

    return { wasmReady, generateThumbnail };
}