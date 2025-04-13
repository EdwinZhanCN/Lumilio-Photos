// hooks/useWasm.jsx
import { useState, useEffect, useRef } from 'react';
import initThumbnail, { generate_thumbnail } from '@/wasm/thumbnail_wasm';
import initBlake3, { hash_asset, verify_asset_hash, compare_assets } from '@/wasm/blake3_wasm';

/**
 * Custom hook to initialize and use WASM modules for thumbnail generation and file hashing.
 * The useWasm() hook can only be used in JSX components, not in regular JavaScript files.
 * @returns any
 */
export function useWasm() {
    const [wasmReady, setWasmReady] = useState(false);
    const initPromise = useRef(null);

    useEffect(() => {
        // Create a singleton initialization promise for both WASM modules
        if (!initPromise.current) {
            initPromise.current = Promise.all([
                initThumbnail(),
                initBlake3()
            ])
                .then(() => {
                    console.log('WASM modules initialized successfully');
                    setWasmReady(true);
                })
                .catch(err => {
                    console.error('Failed to initialize WASM modules:', err);
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

    const hashFile = async (file) => {
        // Ensure WASM is initialized before proceeding
        if (!wasmReady) {
            await initPromise.current;
        }

        const arrayBuffer = await file.arrayBuffer();
        const result = hash_asset(new Uint8Array(arrayBuffer));
        return result.hash;
    };

    const verifyHash = async (file, hashString) => {
        // Ensure WASM is initialized before proceeding
        if (!wasmReady) {
            await initPromise.current;
        }

        const arrayBuffer = await file.arrayBuffer();
        return verify_asset_hash(new Uint8Array(arrayBuffer), hashString);
    };

    const compareFiles = async (file1, file2) => {
        // Ensure WASM is initialized before proceeding
        if (!wasmReady) {
            await initPromise.current;
        }

        const arrayBuffer1 = await file1.arrayBuffer();
        const arrayBuffer2 = await file2.arrayBuffer();
        return compare_assets(new Uint8Array(arrayBuffer1), new Uint8Array(arrayBuffer2));
    };

    return {
        wasmReady,
        generateThumbnail,
        hashFile,
        verifyHash,
        compareFiles
    };
}