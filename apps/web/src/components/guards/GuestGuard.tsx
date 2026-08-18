/**
 * GuestGuard — chặn user đã đăng nhập truy cập trang auth (login, register).
 *
 * Cơ chế:
 *   - isPending = true  → hiển thị spinner
 *   - user tồn tại      → redirect sang / (đã đăng nhập rồi)
 *   - user = null       → render children (trang login/register)
 *
 * Khác với AuthGuard: AuthGuard chặn user CHƯA đăng nhập,
 * GuestGuard chặn user ĐÃ đăng nhập.
 */

import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useSession } from "@/hooks/queries/useSession";

interface GuestGuardProps {
  children: React.ReactNode;
}

export const GuestGuard: React.FC<GuestGuardProps> = ({ children }) => {
  const { user, isPending } = useSession();
  const navigate = useNavigate();

  // Redirect nếu đã có session — useEffect LUÔN được gọi (Rules of Hooks)
  useEffect(() => {
    if (!isPending && user) {
      navigate("/", { replace: true });
    }
  }, [isPending, user, navigate]);

  // Đang kiểm tra session → spinner
  if (isPending) {
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

  // Đã có session → render null (useEffect trên sẽ redirect)
  if (user) {
    return null;
  }

  // Chưa đăng nhập → cho vào trang login/register
  return <>{children}</>;
};

export default GuestGuard;
