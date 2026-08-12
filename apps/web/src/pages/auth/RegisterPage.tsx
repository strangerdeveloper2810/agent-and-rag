import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  SparklesIcon,
  EyeIcon,
  EyeSlashIcon,
  ArrowRightIcon,
  ShieldCheckIcon,
  BoltIcon,
} from "@heroicons/react/24/outline";

import { useAuthStore } from "@/stores/auth.store";
import { useToast } from "@/shared/components/Toast";
import { registerSchema, type RegisterFormValues } from "@/lib/validation";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import type { ApiError } from "@/lib/http";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";

const checkPasswordStrength = (pass: string) => {
  let score = 0;
  if (!pass) return { score: 0, label: "Trống", color: "var(--border)" };
  if (pass.length >= 8) score++;
  if (/[A-Z]/.test(pass) || /[a-z]/.test(pass)) score++;
  if (/[0-9]/.test(pass)) score++;
  if (/[^A-Za-z0-9]/.test(pass)) score++;

  if (score <= 1) return { score: 1, label: "Yếu", color: "#f87171" };
  if (score === 2 || score === 3)
    return { score: 2, label: "Trung bình", color: "#fbbf24" };
  return { score: 3, label: "Mạnh", color: "#34d399" };
};

export const RegisterPage: React.FC = () => {
  const navigate = useNavigate();
  const { register: registerUser, isLoading } = useAuthStore();
  const toast = useToast();
  useDocumentTitle("Đăng ký tài khoản - J.A.R.V.I.S.");

  const [showPassword, setShowPassword] = useState(false);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: { name: "", email: "", password: "" },
  });

  const passwordValue = watch("password") || "";
  const pwdStrength = checkPasswordStrength(passwordValue);

  const onSubmit = async (data: RegisterFormValues) => {
    try {
      await registerUser(data.email.trim(), data.password, data.name.trim());
      toast.success("Tạo tài khoản thành công!");
      navigate("/", { replace: true });
    } catch (err) {
      const apiErr = err as ApiError;
      toast.error(apiErr?.message ?? "Đăng ký thất bại. Email có thể đã tồn tại.");
    }
  };

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      {/* ── Left: Form Panel (Shadcn UI) ── */}
      <div className="flex flex-1 items-center justify-center px-6 sm:px-12 py-10 order-2 lg:order-1">
        <div className="w-full max-w-[380px] animate-slide-up">
          {/* Mobile brand */}
          <div className="mb-8 flex items-center gap-3 lg:hidden">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-md">
              <SparklesIcon className="h-5 w-5" />
            </div>
            <span className="font-display text-lg font-bold text-foreground">
              J.A.R.V.I.S.
            </span>
          </div>

          <Card className="border-border bg-card/80 shadow-2xl">
            <CardHeader className="space-y-1 pb-4">
              <CardTitle className="text-2xl font-bold">Tạo tài khoản mới</CardTitle>
              <CardDescription>
                Bắt đầu trải nghiệm sức mạnh trợ lý AI cá nhân hóa ngay hôm nay
              </CardDescription>
            </CardHeader>

            <CardContent>
              <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
                <div>
                  <label htmlFor="register-name" className="mb-1.5 block text-xs font-medium text-muted-foreground">
                    Họ và tên
                  </label>
                  <Input
                    id="register-name"
                    type="text"
                    autoComplete="name"
                    placeholder="Nguyễn Văn A"
                    {...register("name")}
                  />
                  {errors.name && (
                    <p className="mt-1.5 text-[11px] font-medium text-destructive">
                      {errors.name.message}
                    </p>
                  )}
                </div>

                <div>
                  <label htmlFor="register-email" className="mb-1.5 block text-xs font-medium text-muted-foreground">
                    Địa chỉ Email
                  </label>
                  <Input
                    id="register-email"
                    type="email"
                    autoComplete="email"
                    placeholder="name@company.com"
                    {...register("email")}
                  />
                  {errors.email && (
                    <p className="mt-1.5 text-[11px] font-medium text-destructive">
                      {errors.email.message}
                    </p>
                  )}
                </div>

                <div>
                  <label htmlFor="register-password" className="mb-1.5 block text-xs font-medium text-muted-foreground">
                    Mật khẩu
                  </label>
                  <div className="relative">
                    <Input
                      id="register-password"
                      type={showPassword ? "text" : "password"}
                      autoComplete="new-password"
                      placeholder="Tối thiểu 8 ký tự"
                      className="pr-10"
                      {...register("password")}
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword((p) => !p)}
                      aria-label={showPassword ? "Ẩn mật khẩu" : "Hiện mật khẩu"}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition"
                    >
                      {showPassword ? (
                        <EyeSlashIcon className="h-4 w-4" />
                      ) : (
                        <EyeIcon className="h-4 w-4" />
                      )}
                    </button>
                  </div>

                  {/* Password Strength Indicator */}
                  {passwordValue && (
                    <div className="mt-2.5 space-y-1">
                      <div className="flex justify-between items-center text-[10px]">
                        <span className="text-muted-foreground font-medium">Độ mạnh mật khẩu:</span>
                        <span className="font-bold" style={{ color: pwdStrength.color }}>
                          {pwdStrength.label}
                        </span>
                      </div>
                      <div className="flex gap-1 h-1.5 w-full rounded-full bg-muted overflow-hidden">
                        <div
                          className="h-full transition-all duration-300 rounded-full"
                          style={{
                            width: pwdStrength.score === 1 ? "33%" : pwdStrength.score === 2 ? "66%" : "100%",
                            backgroundColor: pwdStrength.color,
                          }}
                        />
                      </div>
                    </div>
                  )}

                  {errors.password && (
                    <p className="mt-1.5 text-[11px] font-medium text-destructive">
                      {errors.password.message}
                    </p>
                  )}
                </div>

                <Button
                  type="submit"
                  variant="gradient"
                  disabled={isLoading}
                  className="w-full mt-2"
                >
                  {isLoading ? (
                    <span className="inline-flex items-center gap-2">
                      <span className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                      Đang khởi tạo tài khoản...
                    </span>
                  ) : (
                    "Tạo tài khoản J.A.R.V.I.S."
                  )}
                </Button>
              </form>

              {/* Navigation Footer */}
              <div className="mt-6 pt-4 text-center border-t border-border">
                <p className="text-xs text-muted-foreground">
                  Đã có tài khoản?{" "}
                  <Link
                    to="/login"
                    className="font-bold text-primary hover:underline inline-flex items-center gap-1 ml-1"
                  >
                    <span>Đăng nhập ngay</span>
                    <ArrowRightIcon className="h-3 w-3" />
                  </Link>
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* ── Right: Brand Panel ── */}
      <div className="relative hidden w-[48%] flex-col justify-between overflow-hidden bg-card/60 p-10 lg:flex order-1 lg:order-2 border-l border-border backdrop-blur-xl">
        <div className="pointer-events-none absolute -right-20 top-1/3 h-[420px] w-[420px] rounded-full bg-primary/10 blur-[130px]" />

        {/* Top: brand */}
        <div className="relative z-10 flex justify-end">
          <div className="flex items-center gap-3">
            <span className="font-display text-lg font-bold tracking-tight text-foreground">
              J.A.R.V.I.S.
            </span>
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-md">
              <SparklesIcon className="h-5 w-5" />
            </div>
          </div>
        </div>

        {/* Middle: hero */}
        <div className="relative z-10 text-right my-auto py-8">
          <h2 className="font-display text-4xl font-extrabold leading-tight tracking-tight">
            Xây dựng không gian
            <br />
            <span className="bg-gradient-to-r from-indigo-400 via-purple-400 to-pink-400 bg-clip-text text-transparent">
              làm việc thông minh
            </span>
          </h2>
          <p className="ml-auto mt-4 max-w-md text-xs leading-relaxed text-muted-foreground">
            Chỉ với một tài khoản, bạn mở khóa toàn bộ tiềm năng trí tuệ nhân tạo. Tải lên tài liệu, kích hoạt AI Agent và nâng cao hiệu suất làm việc gấp 10 lần.
          </p>

          {/* Highlights */}
          <div className="mt-8 grid grid-cols-2 gap-3 max-w-sm ml-auto text-left">
            <Card className="p-3 bg-background/50 border-border">
              <BoltIcon className="h-5 w-5 text-indigo-400" />
              <h4 className="text-xs font-bold text-foreground mt-1">Phản hồi siêu tốc</h4>
              <p className="text-[10px] text-muted-foreground">SSE Streaming thời gian thực</p>
            </Card>
            <Card className="p-3 bg-background/50 border-border">
              <ShieldCheckIcon className="h-5 w-5 text-emerald-400" />
              <h4 className="text-xs font-bold text-foreground mt-1">Bảo mật tuyệt đối</h4>
              <p className="text-[10px] text-muted-foreground">Mã hóa dữ liệu chuẩn enterprise</p>
            </Card>
          </div>
        </div>

        {/* Bottom */}
        <p className="relative z-10 text-right text-[11px] text-muted-foreground pt-4 border-t border-border">
          © 2026 J.A.R.V.I.S. — Intelligent Agent Platform
        </p>
      </div>
    </div>
  );
};

export default RegisterPage;
