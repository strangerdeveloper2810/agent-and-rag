import React, { useState, useEffect } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import {
  EyeIcon,
  EyeSlashIcon,
  KeyIcon,
  EnvelopeIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  ShieldCheckIcon,
} from "@heroicons/react/24/outline";

import { useAuthStore } from "@/stores/auth.store";
import { useToast } from "@/design-system/molecules/Toast";
import {
  forgotPasswordSchema,
  resetPasswordSchema,
  type ForgotPasswordFormValues,
  type ResetPasswordFormValues,
} from "@/lib/validation";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { translateApiError } from "@/lib/errors";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardHeader,
  CardContent,
} from "@/components/ui/card";

export const ForgotPasswordPage: React.FC = () => {
  const navigate = useNavigate();
  const { forgotPassword, resetPassword, isLoading } = useAuthStore();
  const toast = useToast();
  const { t } = useTranslation("auth");

  useDocumentTitle(
    t("forgotPassword.pageTitle"),
    t("forgotPassword.pageDescription"),
  );

  const [step, setStep] = useState<1 | 2>(1);
  const [email, setEmail] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [resendCooldown, setResendCooldown] = useState(0);

  // Countdown timer for resending OTP
  useEffect(() => {
    if (resendCooldown <= 0) return;
    const timer = setInterval(() => {
      setResendCooldown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [resendCooldown]);

  // Step 1 Form: Email
  const step1Form = useForm<ForgotPasswordFormValues>({
    resolver: zodResolver(forgotPasswordSchema),
    defaultValues: { email: "" },
  });

  // Step 2 Form: OTP + New Password
  const step2Form = useForm<ResetPasswordFormValues>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: { otp: "", newPassword: "", confirmPassword: "" },
  });

  const onStep1Submit = async (data: ForgotPasswordFormValues) => {
    const targetEmail = data.email.trim();
    try {
      await forgotPassword(targetEmail);
      setEmail(targetEmail);
      setStep(2);
      setResendCooldown(60); // 60s cooldown
      toast.success(t("forgotPassword.otpSentSuccess"));
    } catch (err) {
      toast.error(
        translateApiError(
          err,
          t,
          t("forgotPassword.resetFailed"),
        ),
      );
    }
  };

  const onStep2Submit = async (data: ResetPasswordFormValues) => {
    try {
      await resetPassword(email, data.otp.trim(), data.newPassword);
      toast.success(t("forgotPassword.resetSuccess"));
      navigate("/login", { replace: true });
    } catch (err) {
      toast.error(
        translateApiError(
          err,
          t,
          t("forgotPassword.resetFailed"),
        ),
      );
    }
  };

  const handleResendOtp = async () => {
    if (resendCooldown > 0 || !email) return;
    try {
      await forgotPassword(email);
      setResendCooldown(60);
      toast.success(t("forgotPassword.otpSentSuccess"));
    } catch (err) {
      toast.error(
        translateApiError(
          err,
          t,
          t("forgotPassword.resetFailed"),
        ),
      );
    }
  };

  return (
    <div className="min-h-screen bg-background flex flex-col justify-center py-12 px-4 sm:px-6 lg:px-8 relative overflow-hidden">
      {/* Ambient background glow */}
      <div
        className="pointer-events-none absolute -top-40 left-1/2 -translate-x-1/2 w-[720px] h-[360px] bg-primary/8 blur-[140px] rounded-full"
        aria-hidden="true"
      />

      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        {/* Brand header */}
        <div className="flex flex-col items-center mb-8">
          <div className="h-12 w-12 rounded-xl bg-primary/10 border border-primary/20 flex items-center justify-center mb-3 shadow-inner">
            <KeyIcon className="h-6 w-6 text-primary" />
          </div>
          <h2 className="text-2xl font-bold text-foreground text-center">
            {t("forgotPassword.cardTitle")}
          </h2>
          <p className="text-xs text-muted-foreground text-center mt-1 max-w-xs">
            {t("forgotPassword.cardDescription")}
          </p>
        </div>

        <Card className="border-border shadow-xl backdrop-blur-sm bg-card/80">
          <CardHeader className="space-y-1 pb-4">
            <div className="flex items-center justify-between">
              <Badge variant="outline" className="text-[11px] font-medium">
                {step === 1
                  ? t("forgotPassword.step1Title")
                  : t("forgotPassword.step2Title")}
              </Badge>
              <span className="text-[11px] text-muted-foreground font-mono">
                {step}/2
              </span>
            </div>
          </CardHeader>

          <CardContent>
            {step === 1 ? (
              <form
                onSubmit={step1Form.handleSubmit(onStep1Submit)}
                className="space-y-4"
                noValidate
              >
                <div>
                  <label
                    htmlFor="forgot-email"
                    className="mb-1.5 block text-xs font-medium text-muted-foreground"
                  >
                    {t("forgotPassword.emailLabel")}
                  </label>
                  <div className="relative">
                    <Input
                      id="forgot-email"
                      type="email"
                      autoComplete="email"
                      placeholder={t("forgotPassword.emailPlaceholder")}
                      className="pl-9"
                      {...step1Form.register("email")}
                    />
                    <EnvelopeIcon className="h-4 w-4 text-muted-foreground absolute left-3 top-1/2 -translate-y-1/2" />
                  </div>
                  {step1Form.formState.errors.email && (
                    <p className="mt-1.5 text-[11px] font-medium text-destructive">
                      {step1Form.formState.errors.email.message}
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
                      {t("forgotPassword.submittingOtp")}
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-2">
                      <span>{t("forgotPassword.sendOtpButton")}</span>
                      <ArrowRightIcon className="h-4 w-4" />
                    </span>
                  )}
                </Button>
              </form>
            ) : (
              <form
                onSubmit={step2Form.handleSubmit(onStep2Submit)}
                className="space-y-4"
                noValidate
              >
                <div className="p-2.5 rounded-lg bg-muted/40 border border-border/50 flex items-center justify-between text-xs">
                  <span className="text-muted-foreground truncate max-w-[200px]">
                    {email}
                  </span>
                  <button
                    type="button"
                    onClick={() => setStep(1)}
                    className="text-primary hover:underline text-[11px] font-medium"
                  >
                    Đổi email
                  </button>
                </div>

                <div>
                  <label
                    htmlFor="forgot-otp"
                    className="mb-1.5 block text-xs font-medium text-muted-foreground"
                  >
                    {t("forgotPassword.otpLabel")}
                  </label>
                  <Input
                    id="forgot-otp"
                    type="text"
                    maxLength={6}
                    placeholder={t("forgotPassword.otpPlaceholder")}
                    className="font-mono text-center text-lg tracking-widest"
                    {...step2Form.register("otp")}
                  />
                  {step2Form.formState.errors.otp && (
                    <p className="mt-1.5 text-[11px] font-medium text-destructive">
                      {step2Form.formState.errors.otp.message}
                    </p>
                  )}
                </div>

                <div>
                  <label
                    htmlFor="forgot-new-password"
                    className="mb-1.5 block text-xs font-medium text-muted-foreground"
                  >
                    {t("forgotPassword.newPasswordLabel")}
                  </label>
                  <div className="relative">
                    <Input
                      id="forgot-new-password"
                      type={showPassword ? "text" : "password"}
                      placeholder={t("forgotPassword.newPasswordPlaceholder")}
                      className="pr-10"
                      {...step2Form.register("newPassword")}
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword((p) => !p)}
                      aria-label="Toggle password visibility"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition"
                    >
                      {showPassword ? (
                        <EyeSlashIcon className="h-4 w-4" />
                      ) : (
                        <EyeIcon className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                  {step2Form.formState.errors.newPassword && (
                    <p className="mt-1.5 text-[11px] font-medium text-destructive">
                      {step2Form.formState.errors.newPassword.message}
                    </p>
                  )}
                </div>

                <div>
                  <label
                    htmlFor="forgot-confirm-password"
                    className="mb-1.5 block text-xs font-medium text-muted-foreground"
                  >
                    {t("forgotPassword.confirmPasswordLabel")}
                  </label>
                  <div className="relative">
                    <Input
                      id="forgot-confirm-password"
                      type={showConfirmPassword ? "text" : "password"}
                      placeholder={t(
                        "forgotPassword.confirmPasswordPlaceholder",
                      )}
                      className="pr-10"
                      {...step2Form.register("confirmPassword")}
                    />
                    <button
                      type="button"
                      onClick={() => setShowConfirmPassword((p) => !p)}
                      aria-label="Toggle confirm password visibility"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition"
                    >
                      {showConfirmPassword ? (
                        <EyeSlashIcon className="h-4 w-4" />
                      ) : (
                        <EyeIcon className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                  {step2Form.formState.errors.confirmPassword && (
                    <p className="mt-1.5 text-[11px] font-medium text-destructive">
                      {step2Form.formState.errors.confirmPassword.message}
                    </p>
                  )}
                </div>

                <div className="flex items-center justify-between pt-1">
                  <button
                    type="button"
                    disabled={resendCooldown > 0 || isLoading}
                    onClick={handleResendOtp}
                    className="text-xs text-primary hover:underline font-medium disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {resendCooldown > 0
                      ? t("forgotPassword.resendCountdown", {
                          seconds: resendCooldown,
                        })
                      : t("forgotPassword.resendOtp")}
                  </button>
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
                      {t("forgotPassword.submittingReset")}
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-2">
                      <ShieldCheckIcon className="h-4 w-4" />
                      <span>{t("forgotPassword.resetButton")}</span>
                    </span>
                  )}
                </Button>
              </form>
            )}

            {/* Back to Login Footer */}
            <div className="mt-6 pt-4 text-center border-t border-border">
              <Link
                to="/login"
                className="text-xs font-medium text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 transition"
              >
                <ArrowLeftIcon className="h-3 w-3" />
                <span>{t("forgotPassword.backToLogin")}</span>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

export default ForgotPasswordPage;
