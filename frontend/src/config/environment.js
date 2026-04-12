const Environment = {
  backend_url: import.meta.env.VITE_BACKEND_URL ?? "http://localhost:8040",
  frontend_url: import.meta.env.VITE_FRONTEND_URL ?? "http://localhost:3040",
};

export default Environment;