import { create } from "zustand";

const useAuthStore = create((set) => ({
  accessToken: null,

  setAccessToken: (token) => set({ accessToken: token }),

  clearAuth: () => set({ accessToken: null }),
}));

export default useAuthStore;
