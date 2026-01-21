import axios from "axios";

// JWT Token management
const TOKEN_KEY = "auth_token";
const REFRESH_TOKEN_KEY = "refresh_token";

// Token utility functions
export const getToken = () => localStorage.getItem(TOKEN_KEY);
export const getRefreshToken = () => localStorage.getItem(REFRESH_TOKEN_KEY);
export const saveToken = (token: string, refreshToken: string) => {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token);
  }
  if (refreshToken) {
    localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
  }
};
export const removeToken = () => {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
};

// Axios config
const config = {
  baseURL:
    import.meta.env.VITE_API_URL ||
    import.meta.env.API_URL ||
    "http://localhost:8080",
  timeout: 10000,
  withCredentials: true,
};

// Create axios instance
const instance = axios.create(config);

// Request interceptor for adding JWT
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

// Response interceptor for handling errors
instance.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // If 401 error and we haven't tried to refresh the token yet
    if (error.response?.status === 401 && !originalRequest._retry) {
      // Don't retry if it's the refresh token request itself failing
      if (originalRequest.url?.includes("/auth/refresh")) {
        return Promise.reject(error);
      }

      originalRequest._retry = true;

      try {
        // Attempt to refresh the token
        const refreshToken = getRefreshToken();
        if (refreshToken) {
          const res = await axios.post(`${config.baseURL}/api/v1/auth/refresh`, {
            refreshToken,
          });

          // Backend returns code 0 for success
          if (res.data.code === 0 && res.data.data) {
            const { token, refreshToken: newRefreshToken } = res.data.data;
            saveToken(token, newRefreshToken);

            // Update the authorization header
            originalRequest.headers["Authorization"] = `Bearer ${token}`;
            return instance(originalRequest);
          }
        }
      } catch (refreshError) {
        console.error("Token refresh failed:", refreshError);
        removeToken();
      }
    }

    return Promise.reject(error);
  },
);

export default instance;
