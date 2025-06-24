import axios from "axios";

// JWT Token management
const TOKEN_KEY = "auth_token";
const REFRESH_TOKEN_KEY = "refresh_token";

// Token utility functions
export const getToken = () => localStorage.getItem(TOKEN_KEY);
export const getRefreshToken = () => localStorage.getItem(REFRESH_TOKEN_KEY);
export const saveToken = (token: string, refreshToken: string) => {
  localStorage.setItem(TOKEN_KEY, token);
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
  baseURL: import.meta.env.VITE_API_URL || "http://localhost:8080",
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
      originalRequest._retry = true;

      try {
        // Attempt to refresh the token
        const refreshToken = getRefreshToken();
        if (refreshToken) {
          const res = await axios.post(`${config.baseURL}/auth/refresh`, {
            refreshToken,
          });

          const { token } = res.data;
          saveToken(token, refreshToken);

          // Update the authorization header
          originalRequest.headers["Authorization"] = `Bearer ${token}`;
          return instance(originalRequest);
        }
      } catch (error) {
        // If refresh fails, remove tokens and redirect to log in
        removeToken();
      }
    }

    return Promise.reject(error);
  },
);

export default instance;
