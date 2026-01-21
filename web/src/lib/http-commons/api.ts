import axios from "axios";

// JWT Token management
const TOKEN_KEY = "auth_token";
const REFRESH_TOKEN_KEY = "refresh_token";

// Token utility functions
export const getToken = () => localStorage.getItem(TOKEN_KEY);
export const getRefreshToken = () => localStorage.getItem(REFRESH_TOKEN_KEY);
export const saveToken = (token: string, refreshToken: string) => {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  if (refreshToken) localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
};
export const removeToken = () => {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
};

// Axios config
const config = {
  baseURL: import.meta.env.VITE_API_URL || "http://localhost:8080",
  timeout: 15000,
  withCredentials: true,
};

const instance = axios.create(config);

// Request interceptor
instance.interceptors.request.use(
  (config) => {
    const token = getToken();
    if (token) {
      config.headers["Authorization"] = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error),
);

// Response interceptor
instance.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Handle 401 Unauthorized
    if (error.response?.status === 401 && !originalRequest._retry) {
      // Prevent infinite loops on auth endpoints
      if (originalRequest.url?.includes("/auth/refresh") || originalRequest.url?.includes("/auth/login")) {
        return Promise.reject(error);
      }

      originalRequest._retry = true;

      try {
        const refreshToken = getRefreshToken();
        if (refreshToken) {
          // Use axios directly to avoid interceptor loop
          const res = await axios.post(`${config.baseURL}/api/v1/auth/refresh`, {
            refreshToken,
          });

          if (res.data.code === 0 && res.data.data) {
            const { token, refreshToken: newRefreshToken } = res.data.data;
            saveToken(token, newRefreshToken);
            
            // Retry original request with new token
            originalRequest.headers["Authorization"] = `Bearer ${token}`;
            return instance(originalRequest);
          }
        }
      } catch (refreshError) {
        removeToken();
        // Force reload to trigger ProtectedRoute redirect
        window.location.href = "/login";
        return Promise.reject(refreshError);
      }
    }

    return Promise.reject(error);
  },
);

export default instance;
