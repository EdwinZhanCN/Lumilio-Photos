import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { expose } from '@/workers/thumbnail.worker';

// Mock WASM module
vi.mock('@/wasm/thumbnail_wasm', () => ({
    default: vi.fn().mockResolvedValue(undefined),
    generate_thumbnail: vi.fn().mockImplementation((buffer) => {
        // Return a simple mock buffer representing a JPEG
        return new Uint8Array([0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46]);
    })
}));

// Mock for URL.createObjectURL
global.URL.createObjectURL = vi.fn().mockImplementation(() => 'mock-blob-url');

describe('Thumbnail Worker', () => {
    let self;
    let postMessageSpy;

    beforeEach(() => {
        // Mock self (worker global scope)
        postMessageSpy = vi.fn();
        self = {
            postMessage: postMessageSpy,
            onmessage: null
        };

        // Expose worker functionality to our mocked self
        expose(self);
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    it('should initialize WASM when receiving INIT_WASM message', async () => {
        // Trigger onmessage with INIT_WASM
        await self.onmessage({ data: { type: 'INIT_WASM' } });

        // Verify WASM_READY was posted back
        expect(postMessageSpy).toHaveBeenCalledWith({ type: 'WASM_READY' });
    });

    it('should generate thumbnails for image files', async () => {
        // First initialize WASM
        await self.onmessage({ data: { type: 'INIT_WASM' } });

        // Create mock image file
        const mockImageFile = new File(['mock image data'], 'test.jpg', { type: 'image/jpeg' });
        Object.defineProperty(mockImageFile, 'arrayBuffer', {
            value: vi.fn().mockResolvedValue(new ArrayBuffer(8))
        });

        // Trigger thumbnail generation
        await self.onmessage({
            data: {
                type: 'GENERATE_THUMBNAIL',
                id: 'test-id',
                data: {
                    files: [mockImageFile],
                    batchIndex: 0,
                    startIndex: 0
                }
            }
        });

        // Verify progress was reported
        expect(postMessageSpy).toHaveBeenCalledWith(expect.objectContaining({
            type: 'PROGRESS',
            id: 'test-id',
            payload: expect.objectContaining({
                processed: 1,
                total: 1
            })
        }));

        // Verify batch completion was reported
        expect(postMessageSpy).toHaveBeenCalledWith(expect.objectContaining({
            type: 'BATCH_COMPLETE',
            id: 'test-id',
            payload: expect.objectContaining({
                batchIndex: 0,
                results: [{ index: 0, url: 'mock-blob-url' }]
            })
        }));
    });

    it('should create placeholder for video files', async () => {
        // Initialize WASM first
        await self.onmessage({ data: { type: 'INIT_WASM' } });

        // Create mock video file
        const mockVideoFile = new File(['mock video data'], 'test.mp4', { type: 'video/mp4' });

        // Trigger thumbnail generation
        await self.onmessage({
            data: {
                type: 'GENERATE_THUMBNAIL',
                id: 'test-id',
                data: {
                    files: [mockVideoFile],
                    batchIndex: 0,
                    startIndex: 0
                }
            }
        });

        // Verify a batch complete message was sent with the correct URL
        expect(postMessageSpy).toHaveBeenCalledWith(expect.objectContaining({
            type: 'BATCH_COMPLETE',
            payload: expect.objectContaining({
                results: [{ index: 0, url: 'mock-blob-url' }]
            })
        }));
    });

    it('should create placeholder for RAW files', async () => {
        // Initialize WASM first
        await self.onmessage({ data: { type: 'INIT_WASM' } });

        // Create mock RAW file
        const mockRawFile = new File(['mock raw data'], 'test.cr2', { type: 'application/octet-stream' });

        // Trigger thumbnail generation
        await self.onmessage({
            data: {
                type: 'GENERATE_THUMBNAIL',
                id: 'test-id',
                data: {
                    files: [mockRawFile],
                    batchIndex: 0,
                    startIndex: 0
                }
            }
        });

        // Verify a batch complete message was sent
        expect(postMessageSpy).toHaveBeenCalledWith(expect.objectContaining({
            type: 'BATCH_COMPLETE',
            payload: expect.objectContaining({
                results: [{ index: 0, url: 'mock-blob-url' }]
            })
        }));
    });

    it('should handle errors during thumbnail generation', async () => {
        // Initialize WASM first
        await self.onmessage({ data: { type: 'INIT_WASM' } });

        // Create mock file that will throw an error during processing
        const mockErrorFile = new File(['bad data'], 'error.jpg', { type: 'image/jpeg' });
        Object.defineProperty(mockErrorFile, 'arrayBuffer', {
            value: vi.fn().mockRejectedValue(new Error('Mock error'))
        });

        // Trigger thumbnail generation
        await self.onmessage({
            data: {
                type: 'GENERATE_THUMBNAIL',
                id: 'test-id',
                data: {
                    files: [mockErrorFile],
                    batchIndex: 0,
                    startIndex: 0
                }
            }
        });

        // Verify default preview was created (batch still completes)
        expect(postMessageSpy).toHaveBeenCalledWith(expect.objectContaining({
            type: 'BATCH_COMPLETE',
            payload: expect.objectContaining({
                results: [{ index: 0, url: 'mock-blob-url' }]
            })
        }));
    });

    it('should report error if WASM is not initialized', async () => {
        // Skip WASM initialization

        // Trigger thumbnail generation without initializing WASM
        await self.onmessage({
            data: {
                type: 'GENERATE_THUMBNAIL',
                id: 'test-id',
                data: {
                    files: [new File(['data'], 'test.jpg', { type: 'image/jpeg' })],
                    batchIndex: 0,
                    startIndex: 0
                }
            }
        });

        // Verify error message was sent
        expect(postMessageSpy).toHaveBeenCalledWith(
            expect.objectContaining({
                type: 'ERROR',
                error: 'WASM not initialized'
            })
        );
    });
});