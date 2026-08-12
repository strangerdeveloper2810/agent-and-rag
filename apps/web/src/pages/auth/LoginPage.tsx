import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  SparklesIcon,
  BoltIcon,
  EyeIcon,
  EyeSlashIcon,
  CpuChipIcon,
  MagnifyingGlassIcon,
  DocumentTextIcon,
  CodeBracketIcon,
  ArrowRightIcon,
  CheckCircleIcon,
} from "@heroicons/react/24/outline";

import { useAuthStore } from "@/stores/auth.store";
import { useToast } from "@/shared/components/Toast";
import { loginSchema, type LoginFormValues } from "@/lib/validation";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import type { ApiError } from "@/lib/http";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";

const FEATURE_SPOTLIGHTS = [
  {
    title: "Multi-Agent Intelligence",
    desc: "Tự động phân công công việc cho các AI Agent chuyên biệt: phân tích dữ liệu, nghiên cứu web và thực thi code.",
    icon: CpuChipIcon,
    badge: "Multi-Model",
  },
  {
    title: "Hybrid RAG Knowledge",
    desc: "Truy vấn dữ liệu doanh nghiệp và tài liệu cá nhân cực nhanh với độ chính xác cao dựa trên Vector Search.",
    icon: MagnifyingGlassIcon,
    badge: "Vector RAG",
  },
  {
    title: "Multimodal Processing",
    desc: "Đọc, phân tích hình ảnh, file PDF, Excel, Word và đoạn mã ngay trong luồng hội thoại tự nhiên.",
    icon: DocumentTextIcon,
    badge: "Vision & Files",
  },
  {
    title: "Sandboxed Code Execution",
    desc: "Môi trường lập trình an toàn, thực thi script và kết xuất dữ liệu trực quan theo thời gian thực.",
    icon: CodeBracketIcon,
    badge: "Code Runner",
  },
];

export const LoginPage: React.FC = () => {
  const navigate = useNavigate();
  const { login, isLoading } = useAuthStore();
  const toast = useToast();
  useDocumentTitle("Đăng nhập - J.A.R.V.I.S.");

  const [showPassword, setShowPassword] = useState(false);
  const [activeFeature, setActiveFeature] = useState(0);

  const {
    register,
    handleSubmit,
    setValue,
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
      toast.error(apiErr?.message ?? "Đăng nhập thất bại. Vui lòng kiểm tra lại email/mật khẩu.");
    }
  };

  const handleDemoLogin = async () => {
    setValue("email", "demo@javis.ai");
    setValue("password", "password123");
    try {
      await login("demo@javis.ai", "password123");
      toast.success("Đăng nhập tài khoản Demo thành công!");
      navigate("/", { replace: true });
    } catch {
      toast.error("Không thể tự động đăng nhập demo. Vui lòng thử nhập thủ công.");
    }
  };

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      {/* ── Left: Brand & AI Neural Panel ── */}
      <div className="relative hidden w-[48%] flex-col justify-between overflow-hidden bg-card/60 p-10 lg:flex border-r border-border backdrop-blur-xl">
        {/* Radial background glow */}
        <div className="pointer-events-none absolute -left-20 top-1/4 h-[450px] w-[450px] rounded-full bg-primary/10 blur-[130px]" />

        {/* Brand Header */}
        <div className="relative z-10 flex items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-gradient-to-br from-indigo-500 to-purple-600 shadow-lg shadow-indigo-500/25">
            <SparklesIcon className="h-6 w-6 text-white" />
          </div>
          <div>
            <span className="font-display text-xl font-extrabold tracking-tight text-foreground">
              J.A.R.V.I.S.
            </span>
            <p className="text-[10px] font-mono tracking-widest text-primary font-bold uppercase">
              Intelligent Agent System
            </p>
          </div>
        </div>

        {/* Middle Interactive Showcase */}
        <div className="relative z-10 my-auto py-8">
          <Badge variant="accent" className="mb-4 gap-1.5 py-1 px-3">
            <SparklesIcon className="h-3.5 w-3.5" />
            <span>Next-Gen AI Workspace Platform</span>
          </Badge>

          <h2 className="font-display text-4xl font-extrabold leading-tight tracking-tight">
            Trợ lý AI Thông Minh
            <br />
            <span className="bg-gradient-to-r from-indigo-400 via-purple-400 to-pink-400 bg-clip-text text-transparent">
              cho công việc của bạn
            </span>
          </h2>
          <p className="mt-3.5 max-w-md text-xs leading-relaxed text-muted-foreground">
            Hợp nhất đa mô hình trí tuệ nhân tạo, tra cứu dữ liệu doanh nghiệp và tự động hóa quy trình làm việc thông qua hội thoại tự nhiên.
          </p>

          {/* Feature Spotlight Cards */}
          <div className="mt-8 space-y-3">
            {FEATURE_SPOTLIGHTS.map((feat, idx) => {
              const active = idx === activeFeature;
              const Icon = feat.icon;
              return (
                <Card
                  key={feat.title}
                  onClick={() => setActiveFeature(idx)}
                  className={`cursor-pointer p-3.5 transition-all duration-200 ${
                    active
                      ? "border-primary/50 bg-primary/10 shadow-md ring-1 ring-primary/20"
                      : "hover:bg-muted/50 border-border opacity-70"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-background border border-border text-primary">
                        <Icon className="h-4 w-4" />
                      </div>
                      <h4 className="text-xs font-bold text-foreground">{feat.title}</h4>
                    </div>
                    <Badge variant="outline" className="text-[9px] font-mono">
                      {feat.badge}
                    </Badge>
                  </div>
                  {active && (
                    <p className="mt-2 text-[11px] leading-relaxed text-muted-foreground animate-fade-in pl-11">
                      {feat.desc}
                    </p>
                  )}
                </Card>
              );
            })}
          </div>
        </div>

        {/* Footer info */}
        <div className="relative z-10 flex items-center justify-between text-[11px] text-muted-foreground pt-4 border-t border-border">
          <span>© 2026 J.A.R.V.I.S. AI Platform</span>
          <span className="flex items-center gap-1.5 font-mono text-[10px]">
            <CheckCircleIcon className="h-3.5 w-3.5 text-emerald-400" />
            System Online v2.4
          </span>
        </div>
      </div>

      {/* ── Right: Login Form Panel (Shadcn UI components) ── */}
      <div className="flex flex-1 items-center justify-center px-6 sm:px-12 py-10">
        <div className="w-full max-w-[380px] animate-slide-up">
          {/* Mobile Brand */}
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
              <CardTitle className="text-2xl font-bold">Đăng nhập tài khoản</CardTitle>
              <CardDescription>
                Nhập email và mật khẩu của bạn để truy cập không gian làm việc
              </CardDescription>
            </CardHeader>

            <CardContent className="space-y-4">
              {/* 1-Click Quick Demo Login Button */}
              <Button
                type="button"
                variant="outline"
                onClick={handleDemoLogin}
                disabled={isLoading}
                className="w-full gap-2 border-dashed border-primary/40 bg-primary/5 text-primary hover:bg-primary hover:text-white"
              >
                <BoltIcon className="h-4 w-4" />
                <span>Đăng nhập nhanh với tài khoản Demo</span>
              </Button>

              <div className="relative my-4 flex items-center justify-center">
                <div className="w-full border-t border-border" />
                <span className="absolute bg-card px-2.5 text-[10px] font-semibold text-muted-foreground uppercase">
                  hoặc nhập email
                </span>
              </div>

              <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
                <div>
                  <label htmlFor="login-email" className="mb-1.5 block text-xs font-medium text-muted-foreground">
                    Địa chỉ Email
                  </label>
                  <Input
                    id="login-email"
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
                  <label htmlFor="login-password" className="mb-1.5 block text-xs font-medium text-muted-foreground">
                    Mật khẩu
                  </label>
                  <div className="relative">
                    <Input
                      id="login-password"
                      type={showPassword ? "text" : "password"}
                      autoComplete="current-password"
                      placeholder="Nhập mật khẩu của bạn"
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
                      Đang xác thực...
                    </span>
                  ) : (
                    "Đăng nhập ngay"
                  )}
                </Button>
              </form>

              {/* Navigation Footer */}
              <div className="mt-6 pt-4 text-center border-t border-border">
                <p className="text-xs text-muted-foreground">
                  Chưa có tài khoản J.A.R.V.I.S.?{" "}
                  <Link
                    to="/register"
                    className="font-bold text-primary hover:underline inline-flex items-center gap-1 ml-1"
                  >
                    <span>Đăng ký ngay</span>
                    <ArrowRightIcon className="h-3 w-3" />
                  </Link>
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
