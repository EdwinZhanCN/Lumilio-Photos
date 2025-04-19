/// <reference lib="webworker" />

import init, { hash_asset, HashResult } from '@/wasm/blake3_wasm';

let wasmReady = false;
let numberOfFilesProcessed = 0;
let abortController = new AbortController();

/**
 * Worker 接收到的消息数据类型
 */
interface WorkerMessage {
    type: 'ABORT' | 'INIT_WASM' | 'GENERATE_HASH';
    data?: File[];
}

/**
 * Worker 向外发送的错误消息类型
 */
interface ErrorMessage {
    type: 'ERROR';
    payload: {
        error: string;
    };
}

/**
 * 文件哈希结果
 */
interface WorkerHashResult {
    index: number;
    hash: string;
    error?: string;
}

/**
 * 初始化 WASM 模块
 */
async function initialize(): Promise<void> {
    try {
        await init();
        wasmReady = true;
        self.postMessage({ type: 'WASM_READY' });
    } catch (error: unknown) {
        const errMsg = (error as Error).message ?? 'Unknown worker error';
        console.error('Error initializing genHash WebAssembly module:', error);
        const msg: ErrorMessage = {
            type: 'ERROR',
            payload: { error: errMsg },
        };
        self.postMessage(msg);
    }
}

/**
 * 生成多个文件的哈希
 * @param assets - 要处理的文件数组
 * @returns 生成的哈希结果数组（按 index 排序）
 */
async function hashMultipleAssets(assets: File[]): Promise<WorkerHashResult[]> {
    // 动态并发限制，示例根据文件大小决定并发数量
    const CONCURRENCY = assets[0]?.size > 100_000_000 ? 10 : 100;
    const hashResult: WorkerHashResult[] = [];

    for (let i = 0; i < assets.length; i += CONCURRENCY) {
        const batch = assets.slice(i, i + CONCURRENCY);

        // 每批文件并发处理
        const promises = batch.map(async (asset, batchIndex) => {
            const globalIndex = i + batchIndex;
            try {
                let arrayBuffer:ArrayBuffer|null = await asset.arrayBuffer();
                const rawHash: HashResult = hash_asset(new Uint8Array(arrayBuffer));

                // 释放内存
                arrayBuffer = null;

                // 返回对外的简化哈希结果
                return {
                    index: globalIndex,
                    hash: rawHash.hash,
                };
            } catch (err: unknown) {
                const errorMessage = `Error generating hash for [${globalIndex}] ${asset.name}`;
                console.error(errorMessage, err);

                self.postMessage({
                    type: 'ERROR',
                    error: errorMessage,
                });

                return {
                    index: globalIndex,
                    hash: '0'.repeat(64), // 返回一个填充的空哈希
                    error: (err as Error).message,
                };
            } finally {
                // 更新进度
                numberOfFilesProcessed += 1;
                self.postMessage({
                    type: 'PROGRESS',
                    payload: { processed: numberOfFilesProcessed },
                });
            }
        });

        const batchResults = await Promise.all(promises);
        hashResult.push(...batchResults);
    }

    // 最终按 index 排序返回
    return hashResult.sort((a, b) => a.index - b.index);
}

/**
 * 监听主线程发送到 Worker 的消息
 */
self.onmessage = async (e: MessageEvent<WorkerMessage>) => {
    const { type, data } = e.data;

    switch (type) {
        case 'ABORT':
            // 取消任务
            abortController.abort();
            break;

        case 'INIT_WASM':
            // 初始化 WASM
            await initialize();
            break;

        case 'GENERATE_HASH':
            // 生成 Hash
            if (!wasmReady) {
                self.postMessage({ type: 'ERROR', error: 'WASM not initialized' });
                return;
            }
            if (!data) {
                self.postMessage({ type: 'ERROR', error: 'No files provided for hashing' });
                return;
            }
            try {
                const hashResult = await hashMultipleAssets(data);
                self.postMessage({
                    type: 'HASH_COMPLETE',
                    hashResult,
                });
            } catch (err: unknown) {
                console.error('Error generating hash:', err);
                self.postMessage({
                    type: 'ERROR',
                    error: (err as Error).message,
                });
            }
            break;

        default:
            // 未知指令
            self.postMessage({
                type: 'ERROR',
                error: `Unknown message type: ${type}`,
            });
            break;
    }
};