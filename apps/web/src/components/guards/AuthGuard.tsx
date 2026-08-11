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
  const { user, isLoading, init } = useAuthStore();
  const navigate = useNavigate();

  useEffect(() => {
    init();
  }, [init]);

  useEffect(() => {
    if (!isLoading && !user) {
      navigate("/login", { replace: true });
    }
  }, [isLoading, user, navigate]);

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

  if (!user) {
    return null;
  }

  return <>{children}</>;
};

export default AuthGuard;
