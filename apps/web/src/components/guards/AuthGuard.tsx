/**
 * AuthGuard — bảo vệ route yêu cầu đăng nhập.
 *
 * Cơ chế:
 *   - isPending = true  → hiển thị spinner (chưa biết có session hay không)
 *   - user = null       → redirect sang /login
 *   - user tồn tại      → render children
 *
 * Session lấy từ useSession() (TanStack Query) chứ không còn tự gọi init()
 * trong useEffect: nhiều guard cùng mount thì vẫn chỉ một request /api/auth/me,
 * và trong staleTime thì điều hướng qua lại không gọi lại lần nào.
 */

import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useSession } from "@/hooks/queries/useSession";

interface AuthGuardProps {
  children: React.ReactNode;
}

export const AuthGuard: React.FC<AuthGuardProps> = ({ children }) => {
  const { user, isPending } = useSession();
  const navigate = useNavigate();

  // Redirect sang /login nếu đã kiểm tra xong mà không có user
  useEffect(() => {
    if (!isPending && !user) {
      navigate("/login", { replace: true });
    }
  }, [isPending, user, navigate]);

  // Lần đầu tải ứng dụng — hiển thị spinner trung tâm
  if (isPending) {
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
