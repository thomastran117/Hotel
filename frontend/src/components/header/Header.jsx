import { useNavigate } from "react-router-dom";
import api from "@axios";
import useAuthStore from "@stores/authStore";

export default function Header() {
  const accessToken = useAuthStore((s) => s.accessToken);
  const clearAuth = useAuthStore((s) => s.clearAuth);
  const navigate = useNavigate();

  const handleLogout = async () => {
    try {
      await api.post("/auth/logout");
    } finally {
      clearAuth();
      navigate("/login");
    }
  };

  return (
    <header className="sticky top-0 z-50 flex items-center justify-between px-6 py-4 border-b border-gray-200 bg-white">
      <span
        className="text-lg font-semibold tracking-tight cursor-pointer"
        onClick={() => navigate("/")}
      >
        Hotel
      </span>

      <nav className="flex items-center gap-6 text-sm text-gray-600">
        <span
          className="hover:text-gray-900 transition-colors cursor-pointer"
          onClick={() => navigate("/search")}
        >
          Search
        </span>
        <span
          className="hover:text-gray-900 transition-colors cursor-pointer"
          onClick={() => navigate("/hotels")}
        >
          Hotels
        </span>
      </nav>

      {accessToken ? (
        <button
          onClick={handleLogout}
          className="text-sm text-gray-600 hover:text-gray-900 transition-colors"
        >
          Logout
        </button>
      ) : (
        <button
          onClick={() => navigate("/login")}
          className="text-sm text-gray-600 hover:text-gray-900 transition-colors"
        >
          Login
        </button>
      )}
    </header>
  );
}
