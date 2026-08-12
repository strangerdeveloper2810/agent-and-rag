/**
 * AuthGuard — bảo vệ route yêu cầu đăng nhập.
 *
 * Cơ chế:
 *   - isLoading = true  → hiển thị spinner (amber, centered)
 *   - user = null       → redirect sang /login
 *   - user tồn tại      → render children
 */

import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "@/stores/auth.store";

interface AuthGuardProps {
  children: React.ReactNode;
}

export const AuthGuard: React.FC<AuthGuardProps> = ({ children }) => {
  const { user, isLoading, initialized, init } = useAuthStore();
  const navigate = useNavigate();

  // Khởi tạo session lần đầu
  useEffect(() => {
    init();
  }, [init]);

  // Redirect sang /login nếu đã kiểm tra xong mà không có user
  useEffect(() => {
    if (initialized && !isLoading && !user) {
      navigate("/login", { replace: true });
    }
  }, [initialized, isLoading, user, navigate]);

  // Lần đầu tải ứng dụng — hiển thị spinner trung tâm
  if (!initialized || isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-[var(--bg)]">
        <div
          className="h-10 w-10 animate-spin rounded-full"
          style={{
            border: "3px solid rgba(0, 240, 255, 0.2)",
            borderTopColor: "var(--accent)",
          }}
        />
      </div>
    );
  }

  // Không có user → ẩn UI tạm thời cho đến khi redirect
  if (!user) {
    return null;
  }

  return <>{children}</>;
};

export default AuthGuard;
