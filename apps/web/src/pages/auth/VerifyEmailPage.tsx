import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  SparklesIcon,
  ShieldCheckIcon,
  ArrowLeftIcon,
} from "@heroicons/react/24/outline";

import { useAuthStore } from "@/stores/auth.store";
import { useToast } from "@/design-system/molecules/Toast";
import {
  verifyEmailOtpSchema,
  type VerifyEmailOtpFormValues,
} from "@/lib/validation";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { ApiError } from "@/lib/http";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";

const COOLDOWN_SECONDS = 120;

export const VerifyEmailPage: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const email = searchParams.get("email") ?? "";
  const { verifyEmail, resendOtp, isLoading } = useAuthStore();
  const toast = useToast();
  useDocumentTitle("Xác minh email - J.A.R.V.I.S.");

  const [cooldown, setCooldown] = useState(COOLDOWN_SECONDS);
  const [resending, setResending] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<VerifyEmailOtpFormValues>({
    resolver: zodResolver(verifyEmailOtpSchema),
    defaultValues: { otp: "" },
  });

  // Chưa có email trên URL (vào thẳng /verify-email không qua register/login) → về register.
  useEffect(() => {
    if (!email) {
      navigate("/register", { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [email]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => setCooldown((s) => Math.max(0, s - 1)), 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  const onSubmit = async (data: VerifyEmailOtpFormValues) => {
    try {
      await verifyEmail(email, data.otp);
      toast.success("Xác minh thành công!");
      navigate("/", { replace: true });
    } catch (err) {
      const apiErr = err as ApiError;
      toast.error(apiErr?.message ?? "Xác minh thất bại. Vui lòng thử lại.");
    }
  };

  const handleResend = async () => {
    setResending(true);
    try {
      await resendOtp(email);
      toast.success("Đã gửi lại mã OTP!");
      setCooldown(COOLDOWN_SECONDS);
    } catch (err) {
      const apiErr = err as ApiError;
      if (apiErr instanceof ApiError && apiErr.status === 429) {
        // Đồng bộ lại countdown theo server nếu client lệch giờ (vd mở lại tab cũ).
        if (typeof apiErr.retryAfterSeconds === "number") {
          setCooldown(apiErr.retryAfterSeconds);
        }
        toast.error(apiErr.message);
      } else {
        toast.error(apiErr?.message ?? "Gửi lại mã thất bại.");
      }
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-background text-foreground px-6">
      <div className="w-full max-w-[380px] animate-slide-up">
        <div className="mb-8 flex items-center justify-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-md">
            <SparklesIcon className="h-5 w-5" />
          </div>
          <span className="font-display text-lg font-bold text-foreground">
            J.A.R.V.I.S.
          </span>
        </div>

        <Card className="border-border bg-card/80 shadow-2xl">
          <CardHeader className="space-y-1 pb-4 text-center">
            <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/10 text-primary">
              <ShieldCheckIcon className="h-6 w-6" />
            </div>
            <CardTitle className="text-2xl font-bold">Xác minh email</CardTitle>
            <CardDescription>
              Nhập mã 6 số vừa được gửi tới{" "}
              <span className="font-medium text-foreground">{email}</span>
            </CardDescription>
          </CardHeader>

          <CardContent>
            <form
              onSubmit={handleSubmit(onSubmit)}
              className="space-y-4"
              noValidate
            >
              <div>
                <Input
                  id="verify-otp"
                  type="text"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={6}
                  placeholder="000000"
                  className="text-center font-mono text-2xl tracking-[0.5em]"
                  {...register("otp")}
                />
                {errors.otp && (
                  <p className="mt-1.5 text-center text-[11px] font-medium text-destructive">
                    {errors.otp.message}
                  </p>
                )}
              </div>

              <Button
                type="submit"
                variant="gradient"
                disabled={isLoading}
                className="w-full"
              >
                {isLoading ? (
                  <span className="inline-flex items-center gap-2">
                    <span className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                    Đang xác minh...
                  </span>
                ) : (
                  "Xác minh"
                )}
              </Button>
            </form>

            <Button
              type="button"
              variant="outline"
              onClick={handleResend}
              disabled={cooldown > 0 || resending}
              className="mt-3 w-full"
            >
              {cooldown > 0
                ? `Gửi lại mã sau ${cooldown}s`
                : resending
                  ? "Đang gửi..."
                  : "Gửi lại mã"}
            </Button>

            <div className="mt-6 pt-4 text-center border-t border-border">
              <Link
                to="/login"
                className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground"
              >
                <ArrowLeftIcon className="h-3 w-3" />
                <span>Quay lại đăng nhập</span>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

export default VerifyEmailPage;
