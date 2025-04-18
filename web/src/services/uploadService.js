import api from '@/https-common';

export const uploadService = {
    // Check if hash exists in BloomFilter
    checkHashInBloomFilter: async (hash) => {
        return await api.post('/api/bloom-filter/check', { hash });
    },

    // Check multiple hashes at once (more efficient)
    batchCheckHashes: async (hashes) => {
        return await api.post('/api/bloom-filter/batch-check', { hashes });
    },

    // Precise database check
    verifyHashInDatabase: async (hash) => {
        return await api.get(`/api/assets/exists/${hash}`);
    },

    /**
     * Upload a file to the server
     * @param file
     * @param hash
     * @returns {Promise<api.AxiosResponse<any>>}
     */
    uploadFile: async (file, hash, p) => {
        const formData = new FormData();
        formData.append('file', file);
        formData.append('hash', hash);

        return await api.post('/api/assets/upload', formData, {
            headers: { 'Content-Type': 'multipart/form-data' }
        });
    }
};