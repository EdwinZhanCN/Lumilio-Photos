import api from '@/https-common';
import {AxiosRequestConfig, AxiosResponse} from "axios";

// TODO: determine the backend response
interface UploadResponse {
    success: boolean;
    message: string;
}


export const uploadService = {
    // Check if hash exists in BloomFilter
    checkHashInBloomFilter: async (hash:string) => {
        return await api.post('/api/bloom-filter/check', {hash});
    },

    // Check multiple hashes at once (more efficient)
    batchCheckHashes: async (hashes:string[]) => {
        return await api.post('/api/bloom-filter/batch-check', {hashes});
    },

    // Precise database check
    verifyHashInDatabase: async (hash:string) => {
        return await api.get(`/api/assets/exists/${hash}`);
    },

    /**
     * Upload a file to the server
     * @param file - The file to upload
     * @param hash - Unique file identifier
     * @param config - Optional Axios config (e.g., onUploadProgress)
     * @returns A promise resolving to an Axios response with UploadResponse
     */
    uploadFile: async (
        file: File,
        hash: string,
        config?: AxiosRequestConfig
    ): Promise<AxiosResponse<UploadResponse>> => {
        const formData = new FormData();
        formData.append('file', file);
        formData.append('hash', hash);

        return api.post<UploadResponse>('/api/assets/upload', formData, {
            ...config,
            headers: {
                'Content-Type': 'multipart/form-data',
                ...(config?.headers ?? {}),
            },
        });
    },

};