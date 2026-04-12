import axios from "axios";
import Environment from "@config/environment";
import useAuthStore from "@stores/authStore";

const api = axios.create({
  baseURL: Environment.backend_url,
  withCredentials: true, // send the refresh token cookie
});

// ── Request interceptor ────────────────────────────────────────────────────────
api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// ── Response interceptor ───────────────────────────────────────────────────────
let isRefreshing = false;
let queue = []; // callbacks waiting for the new token

const processQueue = (error, token = null) => {
  queue.forEach((cb) => (error ? cb.reject(error) : cb.resolve(token)));
  queue = [];
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const original = error.config;

    if (error.response?.status !== 401 || original._retry) {
      return Promise.reject(error);
    }

    if (isRefreshing) {
      // Queue this request until the refresh completes
      return new Promise((resolve, reject) => {
        queue.push({ resolve, reject });
      }).then((token) => {
        original.headers.Authorization = `Bearer ${token}`;
        return api(original);
      });
    }

    original._retry = true;
    isRefreshing = true;

    try {
      const { data } = await axios.post(
        `${Environment.backend_url}/auth/refresh`,
        {},
        { withCredentials: true }
      );

      const newToken = data.access_token;
      useAuthStore.getState().setAccessToken(newToken);
      processQueue(null, newToken);

      original.headers.Authorization = `Bearer ${newToken}`;
      return api(original);
    } catch (refreshError) {
      processQueue(refreshError);
      useAuthStore.getState().clearAuth();
      return Promise.reject(refreshError);
    } finally {
      isRefreshing = false;
    }
  }
);

export default api;
