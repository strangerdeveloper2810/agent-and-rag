/**
 * LoginPage — trang đăng nhập với Luxury Dark theme.
 *
 * Dùng react-hook-form + zod để validation form.
 * Lỗi field hiển thị inline dưới input, lỗi API hiển thị qua toast.
 */

import { Link, useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useAuthStore } from "@/stores/auth.store";
import { useToast } from "@/design-system/molecules/Toast";
import { loginSchema, type LoginFormValues } from "@/lib/validation";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import type { ApiError } from "@/lib/http";

// ── Shared input styles ──

const inputBase =
  "w-full rounded-xl px-4 py-2.5 text-sm outline-none transition-colors placeholder:text-[#475569] bg-[#0b0d14] border border-[#21283b] text-[#f8fafc] focus:border-[#f59e0b]";
const fieldError = "mt-1 text-[11px] text-[#fca5a5]";

export const LoginPage: React.FC = () => {
  const navigate = useNavigate();
  const { login, isLoading } = useAuthStore();
  const toast = useToast();
  useDocumentTitle("Đăng nhập");

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });

  const onSubmit = async (data: LoginFormValues) => {
    try {
      await login(data.email.trim(), data.password);
      toast.success("Đăng nhập thành công!");
      navigate("/", { replace: true });
    } catch (err) {
      const apiErr = err as ApiError;
      toast.error(apiErr?.message ?? "Đăng nhập thất bại.");
    }
  };

  return (
    <div
      className="flex min-h-screen items-center justify-center px-4"
      style={{ backgroundColor: "#0b0d14" }}
    >
      <div
        className="w-full max-w-sm rounded-2xl p-8"
        style={{
          backgroundColor: "#131724",
          border: "1px solid #21283b",
          boxShadow: "0 16px 48px rgba(0, 0, 0, 0.4)",
        }}
      >
        {/* Logo / Brand */}
        <div className="mb-8 text-center">
          <div
            className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl text-xl font-bold"
            style={{
              backgroundColor: "rgba(245, 158, 11, 0.12)",
              color: "#f59e0b",
            }}
          >
            A
          </div>
          <h1
            className="text-xl font-semibold tracking-tight"
            style={{ color: "#f8fafc" }}
          >
            Welcome back
          </h1>
          <p className="mt-1 text-sm" style={{ color: "#94a3b8" }}>
            Sign in to your account
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
          {/* Email */}
          <div>
            <label
              htmlFor="login-email"
              className="mb-1.5 block text-xs font-medium"
              style={{ color: "#94a3b8" }}
            >
              Email
            </label>
            <input
              id="login-email"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              className={inputBase}
              {...register("email")}
            />
            {errors.email && (
              <p className={fieldError}>{errors.email.message}</p>
            )}
          </div>

          {/* Password */}
          <div>
            <label
              htmlFor="login-password"
              className="mb-1.5 block text-xs font-medium"
              style={{ color: "#94a3b8" }}
            >
              Password
            </label>
            <input
              id="login-password"
              type="password"
              autoComplete="current-password"
              placeholder="Enter your password"
              className={inputBase}
              {...register("password")}
            />
            {errors.password && (
              <p className={fieldError}>{errors.password.message}</p>
            )}
          </div>

          {/* Submit */}
          <button
            type="submit"
            disabled={isLoading}
            className="w-full rounded-xl py-2.5 text-sm font-semibold text-white transition-colors disabled:opacity-50"
            style={{ backgroundColor: "#f59e0b" }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "#d97706";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "#f59e0b";
            }}
          >
            {isLoading ? (
              <span className="inline-flex items-center gap-2">
                <span className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                Signing in...
              </span>
            ) : (
              "Sign in"
            )}
          </button>
        </form>

        {/* Register link */}
        <p className="mt-6 text-center text-xs" style={{ color: "#64748b" }}>
          Chưa có tài khoản?{" "}
          <Link
            to="/register"
            className="font-medium transition-colors hover:underline"
            style={{ color: "#f59e0b" }}
          >
            Đăng ký
          </Link>
        </p>
      </div>
    </div>
  );
};

export default LoginPage;
