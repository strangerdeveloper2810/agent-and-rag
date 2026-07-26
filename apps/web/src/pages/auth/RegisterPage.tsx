/**
 * RegisterPage — trang đăng ký tài khoản với Luxury Dark theme.
 *
 * Dùng react-hook-form + zod để validation form.
 * Lỗi field hiển thị inline dưới input, lỗi API hiển thị qua toast.
 */

import { Link, useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useAuthStore } from "@/stores/auth.store";
import { useToast } from "@/shared/components/Toast";
import { registerSchema, type RegisterFormValues } from "@/lib/validation";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import type { ApiError } from "@/lib/http";

// ── Shared input styles ──

const inputBase =
  "w-full rounded-xl px-4 py-2.5 text-sm outline-none transition-colors placeholder:text-[#475569] bg-[#0b0d14] border border-[#21283b] text-[#f8fafc] focus:border-[#f59e0b]";
const fieldError = "mt-1 text-[11px] text-[#fca5a5]";

export const RegisterPage: React.FC = () => {
  const navigate = useNavigate();
  const { register: registerUser, isLoading } = useAuthStore();
  const toast = useToast();
  useDocumentTitle("Đăng ký");

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: { name: "", email: "", password: "" },
  });

  const onSubmit = async (data: RegisterFormValues) => {
    try {
      await registerUser(data.email.trim(), data.password, data.name.trim());
      toast.success("Đăng ký thành công!");
      navigate("/", { replace: true });
    } catch (err) {
      const apiErr = err as ApiError;
      toast.error(apiErr?.message ?? "Đăng ký thất bại.");
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
            Create account
          </h1>
          <p className="mt-1 text-sm" style={{ color: "#94a3b8" }}>
            Get started with your free account
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
          {/* Name */}
          <div>
            <label
              htmlFor="register-name"
              className="mb-1.5 block text-xs font-medium"
              style={{ color: "#94a3b8" }}
            >
              Họ tên
            </label>
            <input
              id="register-name"
              type="text"
              autoComplete="name"
              placeholder="Nguyễn Văn A"
              className={inputBase}
              {...register("name")}
            />
            {errors.name && (
              <p className={fieldError}>{errors.name.message}</p>
            )}
          </div>

          {/* Email */}
          <div>
            <label
              htmlFor="register-email"
              className="mb-1.5 block text-xs font-medium"
              style={{ color: "#94a3b8" }}
            >
              Email
            </label>
            <input
              id="register-email"
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
              htmlFor="register-password"
              className="mb-1.5 block text-xs font-medium"
              style={{ color: "#94a3b8" }}
            >
              Mật khẩu
            </label>
            <input
              id="register-password"
              type="password"
              autoComplete="new-password"
              placeholder="Tối thiểu 8 ký tự"
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
                Creating account...
              </span>
            ) : (
              "Create account"
            )}
          </button>
        </form>

        {/* Login link */}
        <p className="mt-6 text-center text-xs" style={{ color: "#64748b" }}>
          Đã có tài khoản?{" "}
          <Link
            to="/login"
            className="font-medium transition-colors hover:underline"
            style={{ color: "#f59e0b" }}
          >
            Đăng nhập
          </Link>
        </p>
      </div>
    </div>
  );
};

export default RegisterPage;
