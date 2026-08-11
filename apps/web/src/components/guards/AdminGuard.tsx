/**
 * AdminGuard — bảo vệ route yêu cầu quyền admin.
 *
 * Kế thừa hành vi từ AuthGuard:
 *   - Kiểm tra session (init)
 *   - Nếu user.role !== "admin" → hiển thị "Access Denied"
 *   - Nếu là admin → render children
 */

import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "@/stores/auth.store";

interface AdminGuardProps {
  children: React.ReactNode;
}

export const AdminGuard: React.FC<AdminGuardProps> = ({ children }) => {
  const { user, isLoading, init } = useAuthStore();
  const navigate = useNavigate();

  useEffect(() => {
    init();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Đang kiểm tra session → spinner
  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-[var(--bg)]">
        <div
          className="h-10 w-10 animate-spin rounded-full"
          style={{
            border: "3px solid rgba(245, 158, 11, 0.2)",
            borderTopColor: "#f59e0b",
          }}
        />
      </div>
    );
  }

  // Không có session → redirect về login
  if (!user) {
    useEffect(() => {
      navigate("/login", { replace: true });
    });
    return null;
  }

  // Có session nhưng không phải admin
  if (user.role !== "admin") {
    return (
      <div className="flex h-screen items-center justify-center bg-[var(--bg)]">
        <div className="text-center">
          <div className="mb-4 text-6xl">&#x1F6AB;</div>
          <h1 className="text-2xl font-semibold text-[var(--text)]">
            Access Denied
          </h1>
          <p className="mt-2 text-sm text-[var(--text-secondary)]">
            You do not have permission to view this page.
          </p>
          <button
            type="button"
            onClick={() => navigate("/", { replace: true })}
            className="mt-6 rounded-xl px-5 py-2.5 text-sm font-medium text-white transition-colors"
            style={{ backgroundColor: "#f59e0b" }}
          >
            Back to Home
          </button>
        </div>
      </div>
    );
  }

  return <>{children}</>;
};

export default AdminGuard;
