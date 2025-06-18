import api from "@/https-common";
import { AxiosRequestConfig, AxiosResponse } from "axios";

// Response interfaces based on swagger.yaml
interface UploadResponse {
    content_hash: string;
    file_name: string;
    message: string;
    size: number;
    status: string;
    task_id: string;
}

interface BatchUploadResponse {
    results: Record<string, any>[];
}

export const uploadService = {
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
        config?: AxiosRequestConfig,
    ): Promise<AxiosResponse<UploadResponse>> => {
        const formData = new FormData();
        formData.append("file", file);

        // Add the hash as a header instead of form data
        return api.post<UploadResponse>("/api/v1/assets", formData, {
            ...config,
            headers: {
                "Content-Type": "multipart/form-data",
                "X-Content-Hash": hash,
                ...(config?.headers ?? {}),
            },
        });
    },

    /**
     * Batch upload multiple files
     * @param files - Array of files to upload with their hashes
     * @param config - Optional Axios config
     */
    batchUploadFiles: async (
        files: { file: File; hash: string }[],
        config?: AxiosRequestConfig,
    ): Promise<AxiosResponse<BatchUploadResponse>> => {
        const formData = new FormData();

        files.forEach((fileObj) => {
            formData.append(fileObj.hash, fileObj.file, fileObj.file.name);
        });

        return api.post<BatchUploadResponse>("/api/v1/assets/batch", formData, {
            ...config,
            headers: {
                "Content-Type": "multipart/form-data",
                ...(config?.headers ?? {}),
            },
        });
    },
};
