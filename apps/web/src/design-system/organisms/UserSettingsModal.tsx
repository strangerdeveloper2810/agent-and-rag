import React, { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import {
  XMarkIcon,
  UserCircleIcon,
  SparklesIcon,
  ShieldCheckIcon,
  AdjustmentsHorizontalIcon,
  CommandLineIcon,
  BriefcaseIcon,
  LightBulbIcon,
  WrenchScrewdriverIcon,
  CheckIcon,
  ArrowDownTrayIcon,
  DocumentTextIcon,
  SunIcon,
  MoonIcon,
  EyeIcon,
  EyeSlashIcon,
} from "@heroicons/react/24/outline";

import { useAuthStore } from "@/stores/auth.store";
import { useUserStore } from "@/stores/user.store";
import { useToast } from "@/design-system/molecules/Toast";
import { useTheme } from "@/hooks/useTheme";
import { persistLocale, type Locale } from "@/i18n/locale";
import { getMessages } from "@/modules/chat/chat.api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";

interface UserSettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  initialTab?: "profile" | "persona" | "security" | "preferences";
  conversationId?: string;
  conversationMessages?: Array<{ role: string; content: string; createdAt?: string }>;
}

const PRESET_OPTIONS = [
  {
    id: "default" as const,
    icon: SparklesIcon,
    accent: "from-amber-500/20 to-orange-500/20 border-amber-500/30 text-amber-500",
  },
  {
    id: "coder" as const,
    icon: CommandLineIcon,
    accent: "from-emerald-500/20 to-teal-500/20 border-emerald-500/30 text-emerald-500",
  },
  {
    id: "business" as const,
    icon: BriefcaseIcon,
    accent: "from-blue-500/20 to-indigo-500/20 border-blue-500/30 text-blue-500",
  },
  {
    id: "creative" as const,
    icon: LightBulbIcon,
    accent: "from-purple-500/20 to-pink-500/20 border-purple-500/30 text-purple-500",
  },
  {
    id: "custom" as const,
    icon: WrenchScrewdriverIcon,
    accent: "from-slate-500/20 to-zinc-500/20 border-slate-500/30 text-slate-400",
  },
];

export const UserSettingsModal: React.FC<UserSettingsModalProps> = ({
  isOpen,
  onClose,
  initialTab = "persona",
  conversationId,
  conversationMessages = [],
}) => {
  const { t, i18n } = useTranslation("settings");
  const { user } = useAuthStore();
  const {
    fetchSettings,
    updateSettings,
    updateProfile,
    changePassword,
    isSaving,
  } = useUserStore();
  const toast = useToast();
  const { theme, setTheme } = useTheme();

  const locale = (i18n.language === "en" ? "en" : "vi") as Locale;
  const changeLocale = (newLoc: Locale) => {
    i18n.changeLanguage(newLoc);
    persistLocale(newLoc);
  };

  const [activeTab, setActiveTab] = useState<
    "profile" | "persona" | "security" | "preferences"
  >(initialTab);

  // Profile form state
  const [displayName, setDisplayName] = useState(user?.name || "");

  // Persona form state
  const [personaPreset, setPersonaPreset] = useState<
    "default" | "coder" | "business" | "creative" | "custom"
  >("default");
  const [formality, setFormality] = useState<"casual" | "neutral" | "formal">(
    "neutral",
  );
  const [verbosity, setVerbosity] = useState<"concise" | "normal" | "detailed">(
    "normal",
  );
  const [customInstructions, setCustomInstructions] = useState("");

  // Security form state
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showOldPassword, setShowOldPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);

  useEffect(() => {
    if (isOpen) {
      setActiveTab(initialTab);
      setDisplayName(user?.name || "");
      fetchSettings().then((s) => {
        if (s) {
          setPersonaPreset(s.persona_preset || "default");
          setFormality(s.formality || "neutral");
          setVerbosity(s.verbosity || "normal");
          setCustomInstructions(s.custom_instructions || "");
        }
      });
    }
  }, [isOpen, initialTab, user, fetchSettings]);

  if (!isOpen) return null;

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!displayName.trim()) return;
    try {
      await updateProfile({ name: displayName.trim() });
      toast.success(t("profile.success"));
    } catch (err: any) {
      toast.error(err?.message || "Lỗi cập nhật thông tin");
    }
  };

  const handleSavePersona = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await updateSettings({
        persona_preset: personaPreset,
        formality,
        verbosity,
        custom_instructions: customInstructions.trim(),
      });
      toast.success(t("persona.success"));
    } catch (err: any) {
      toast.error(err?.message || "Lỗi lưu cài đặt Persona");
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!oldPassword || !newPassword) {
      toast.error("Vui lòng nhập đầy đủ thông tin");
      return;
    }
    if (newPassword.length < 8) {
      toast.error("Mật khẩu mới phải từ 8 ký tự trở lên");
      return;
    }
    if (newPassword !== confirmPassword) {
      toast.error("Mật khẩu xác nhận không khớp");
      return;
    }
    try {
      await changePassword(oldPassword, newPassword);
      toast.success(t("security.success"));
      setOldPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err: any) {
      toast.error(err?.message || "Đổi mật khẩu thất bại");
    }
  };

  const handleExportMarkdown = async () => {
    let msgs = conversationMessages;
    if (!msgs.length && conversationId) {
      try {
        msgs = (await getMessages(conversationId)) as any;
      } catch {
        // ignore
      }
    }
    if (!msgs.length) {
      toast.error("Không có tin nhắn nào trong cuộc trò chuyện này");
      return;
    }
    const content = msgs
      .map(
        (m: { role: string; content: string }) =>
          `### ${m.role === "user" ? "👤 Bạn" : "🤖 J.A.R.V.I.S."}\n\n${m.content}\n\n---`,
      )
      .join("\n\n");
    const blob = new Blob([content], { type: "text/markdown;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `javis-chat-${conversationId || Date.now()}.md`;
    a.click();
    URL.revokeObjectURL(url);
    toast.success("Đã xuất file Markdown!");
  };

  const handleExportJson = async () => {
    let msgs = conversationMessages;
    if (!msgs.length && conversationId) {
      try {
        msgs = (await getMessages(conversationId)) as any;
      } catch {
        // ignore
      }
    }
    if (!msgs.length) {
      toast.error("Không có tin nhắn nào trong cuộc trò chuyện này");
      return;
    }
    const jsonStr = JSON.stringify(msgs, null, 2);
    const blob = new Blob([jsonStr], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `javis-chat-${conversationId || Date.now()}.json`;
    a.click();
    URL.revokeObjectURL(url);
    toast.success("Đã xuất file JSON!");
  };

  const userInitials = user?.name
    ? user.name
        .split(" ")
        .map((p) => p[0])
        .slice(0, 2)
        .join("")
        .toUpperCase()
    : "US";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-200">
      <div
        className="relative w-full max-w-3xl max-h-[90vh] bg-card border border-border rounded-2xl shadow-2xl overflow-hidden flex flex-col animate-in zoom-in-95 duration-200"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-6 py-4 border-b border-border flex items-center justify-between shrink-0 bg-muted/20">
          <div>
            <h2 className="text-lg font-bold text-foreground">
              {t("modal.title")}
            </h2>
            <p className="text-xs text-muted-foreground">{t("modal.subtitle")}</p>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition"
            aria-label={t("modal.close")}
          >
            <XMarkIcon className="h-5 w-5" />
          </button>
        </div>

        {/* Tabs Bar */}
        <div className="flex items-center gap-2 px-6 border-b border-border bg-background/50 overflow-x-auto shrink-0 py-2">
          <button
            onClick={() => setActiveTab("persona")}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition shrink-0 ${
              activeTab === "persona"
                ? "bg-primary/10 text-primary"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            <SparklesIcon className="h-4 w-4" />
            <span>{t("modal.tabs.persona")}</span>
          </button>
          <button
            onClick={() => setActiveTab("profile")}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition shrink-0 ${
              activeTab === "profile"
                ? "bg-primary/10 text-primary"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            <UserCircleIcon className="h-4 w-4" />
            <span>{t("modal.tabs.profile")}</span>
          </button>
          <button
            onClick={() => setActiveTab("security")}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition shrink-0 ${
              activeTab === "security"
                ? "bg-primary/10 text-primary"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            <ShieldCheckIcon className="h-4 w-4" />
            <span>{t("modal.tabs.security")}</span>
          </button>
          <button
            onClick={() => setActiveTab("preferences")}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition shrink-0 ${
              activeTab === "preferences"
                ? "bg-primary/10 text-primary"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            <AdjustmentsHorizontalIcon className="h-4 w-4" />
            <span>{t("modal.tabs.preferences")}</span>
          </button>
        </div>

        {/* Content Body */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {/* TAB 1: PERSONA */}
          {activeTab === "persona" && (
            <form onSubmit={handleSavePersona} className="space-y-6">
              <div>
                <h3 className="text-sm font-bold text-foreground">
                  {t("persona.title")}
                </h3>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {t("persona.desc")}
                </p>
              </div>

              {/* Persona Presets */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-3">
                  {t("persona.presetLabel")}
                </label>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                  {PRESET_OPTIONS.map((item) => {
                    const isSelected = personaPreset === item.id;
                    const Icon = item.icon;
                    return (
                      <button
                        key={item.id}
                        type="button"
                        onClick={() => setPersonaPreset(item.id)}
                        className={`flex items-start gap-3 p-3 rounded-xl text-left border transition ${
                          isSelected
                            ? "bg-primary/5 border-primary shadow-sm"
                            : "bg-card hover:bg-muted/40 border-border"
                        }`}
                      >
                        <div
                          className={`p-2 rounded-lg bg-gradient-to-br ${item.accent} shrink-0`}
                        >
                          <Icon className="h-4 w-4" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center justify-between">
                            <span className="text-xs font-bold text-foreground">
                              {t(`persona.presets.${item.id}.title`)}
                            </span>
                            {isSelected && (
                              <CheckIcon className="h-4 w-4 text-primary" />
                            )}
                          </div>
                          <p className="text-[11px] text-muted-foreground mt-0.5 line-clamp-2">
                            {t(`persona.presets.${item.id}.desc`)}
                          </p>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Tone & Verbosity Grid */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {/* Formality */}
                <div>
                  <label className="block text-xs font-semibold text-foreground mb-2">
                    {t("persona.formalityLabel")}
                  </label>
                  <div className="flex rounded-lg p-1 bg-muted/50 border border-border gap-1">
                    {(["casual", "neutral", "formal"] as const).map((fmt) => (
                      <button
                        key={fmt}
                        type="button"
                        onClick={() => setFormality(fmt)}
                        className={`flex-1 py-1.5 text-xs font-medium rounded-md transition ${
                          formality === fmt
                            ? "bg-background text-foreground shadow-sm font-bold"
                            : "text-muted-foreground hover:text-foreground"
                        }`}
                      >
                        {t(`persona.formality.${fmt}`)}
                      </button>
                    ))}
                  </div>
                </div>

                {/* Verbosity */}
                <div>
                  <label className="block text-xs font-semibold text-foreground mb-2">
                    {t("persona.verbosityLabel")}
                  </label>
                  <div className="flex rounded-lg p-1 bg-muted/50 border border-border gap-1">
                    {(["concise", "normal", "detailed"] as const).map((vb) => (
                      <button
                        key={vb}
                        type="button"
                        onClick={() => setVerbosity(vb)}
                        className={`flex-1 py-1.5 text-xs font-medium rounded-md transition ${
                          verbosity === vb
                            ? "bg-background text-foreground shadow-sm font-bold"
                            : "text-muted-foreground hover:text-foreground"
                        }`}
                      >
                        {t(`persona.verbosity.${vb}`)}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Custom Instructions */}
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <label className="block text-xs font-semibold text-foreground">
                    {t("persona.customInstructionsLabel")}
                  </label>
                  <span className="text-[11px] text-muted-foreground">
                    {customInstructions.length}/2000
                  </span>
                </div>
                <textarea
                  rows={4}
                  maxLength={2000}
                  value={customInstructions}
                  onChange={(e) => setCustomInstructions(e.target.value)}
                  placeholder={t("persona.customInstructionsPlaceholder")}
                  className="w-full rounded-xl bg-background border border-border p-3 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary transition resize-none"
                />
                <p className="text-[11px] text-muted-foreground mt-1">
                  {t("persona.customInstructionsHint")}
                </p>
              </div>

              <div className="flex justify-end pt-2">
                <Button type="submit" variant="gradient" disabled={isSaving}>
                  {isSaving ? t("modal.saving") : t("modal.save")}
                </Button>
              </div>
            </form>
          )}

          {/* TAB 2: PROFILE */}
          {activeTab === "profile" && (
            <form onSubmit={handleSaveProfile} className="space-y-5">
              <div>
                <h3 className="text-sm font-bold text-foreground">
                  {t("profile.title")}
                </h3>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {t("profile.desc")}
                </p>
              </div>

              <div className="flex items-center gap-4 p-4 rounded-xl bg-muted/20 border border-border">
                <Avatar className="h-16 w-16">
                  <AvatarFallback className="bg-gradient-to-br from-indigo-500 to-purple-600 text-white font-bold text-lg">
                    {userInitials}
                  </AvatarFallback>
                </Avatar>
                <div>
                  <h4 className="text-sm font-bold text-foreground">
                    {user?.name || "Người dùng"}
                  </h4>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    {user?.email}
                  </p>
                  <Badge variant="outline" className="mt-2 text-[10px] uppercase">
                    {user?.role === "admin"
                      ? t("profile.adminRole")
                      : t("profile.userRole")}
                  </Badge>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  {t("profile.nameLabel")}
                </label>
                <Input
                  type="text"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder={t("profile.namePlaceholder")}
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  {t("profile.emailLabel")}
                </label>
                <Input type="email" value={user?.email || ""} disabled />
              </div>

              <div className="flex justify-end pt-2">
                <Button type="submit" variant="gradient" disabled={isSaving}>
                  {isSaving ? t("modal.saving") : t("modal.save")}
                </Button>
              </div>
            </form>
          )}

          {/* TAB 3: SECURITY */}
          {activeTab === "security" && (
            <form onSubmit={handleChangePassword} className="space-y-4">
              <div>
                <h3 className="text-sm font-bold text-foreground">
                  {t("security.title")}
                </h3>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {t("security.desc")}
                </p>
              </div>

              <div>
                <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  {t("security.oldPasswordLabel")}
                </label>
                <div className="relative">
                  <Input
                    type={showOldPassword ? "text" : "password"}
                    value={oldPassword}
                    onChange={(e) => setOldPassword(e.target.value)}
                    placeholder={t("security.oldPasswordPlaceholder")}
                    className="pr-10"
                  />
                  <button
                    type="button"
                    onClick={() => setShowOldPassword((v) => !v)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  >
                    {showOldPassword ? (
                      <EyeSlashIcon className="h-4 w-4" />
                    ) : (
                      <EyeIcon className="h-4 w-4" />
                    )}
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  {t("security.newPasswordLabel")}
                </label>
                <div className="relative">
                  <Input
                    type={showNewPassword ? "text" : "password"}
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder={t("security.newPasswordPlaceholder")}
                    className="pr-10"
                  />
                  <button
                    type="button"
                    onClick={() => setShowNewPassword((v) => !v)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  >
                    {showNewPassword ? (
                      <EyeSlashIcon className="h-4 w-4" />
                    ) : (
                      <EyeIcon className="h-4 w-4" />
                    )}
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  {t("security.confirmPasswordLabel")}
                </label>
                <Input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder={t("security.confirmPasswordPlaceholder")}
                />
              </div>

              <div className="flex justify-end pt-2">
                <Button type="submit" variant="gradient" disabled={isSaving}>
                  {isSaving ? t("modal.saving") : t("security.changePasswordButton")}
                </Button>
              </div>
            </form>
          )}

          {/* TAB 4: PREFERENCES */}
          {activeTab === "preferences" && (
            <div className="space-y-6">
              <div>
                <h3 className="text-sm font-bold text-foreground">
                  {t("preferences.title")}
                </h3>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {t("preferences.desc")}
                </p>
              </div>

              {/* Theme Selection */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-2">
                  {t("preferences.themeLabel")}
                </label>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    onClick={() => setTheme("dark")}
                    className={`flex items-center gap-3 p-3 rounded-xl border transition ${
                      theme === "dark"
                        ? "bg-primary/10 border-primary text-foreground font-bold"
                        : "bg-card hover:bg-muted/40 border-border text-muted-foreground"
                    }`}
                  >
                    <MoonIcon className="h-5 w-5 text-indigo-400" />
                    <span className="text-xs">{t("preferences.theme.dark")}</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setTheme("light")}
                    className={`flex items-center gap-3 p-3 rounded-xl border transition ${
                      theme === "light"
                        ? "bg-primary/10 border-primary text-foreground font-bold"
                        : "bg-card hover:bg-muted/40 border-border text-muted-foreground"
                    }`}
                  >
                    <SunIcon className="h-5 w-5 text-amber-500" />
                    <span className="text-xs">{t("preferences.theme.light")}</span>
                  </button>
                </div>
              </div>

              {/* Language Selection */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-2">
                  {t("preferences.languageLabel")}
                </label>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => changeLocale("vi")}
                    className={`flex items-center gap-2 px-4 py-2 rounded-xl border text-xs font-medium transition ${
                      locale === "vi"
                        ? "bg-primary/10 border-primary text-primary font-bold"
                        : "bg-card hover:bg-muted/40 border-border text-foreground"
                    }`}
                  >
                    <span>🇻🇳 Tiếng Việt</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => changeLocale("en")}
                    className={`flex items-center gap-2 px-4 py-2 rounded-xl border text-xs font-medium transition ${
                      locale === "en"
                        ? "bg-primary/10 border-primary text-primary font-bold"
                        : "bg-card hover:bg-muted/40 border-border text-foreground"
                    }`}
                  >
                    <span>🇺🇸 English</span>
                  </button>
                </div>
              </div>

              {/* Export Chat */}
              <div className="pt-4 border-t border-border">
                <label className="block text-xs font-semibold text-foreground mb-2">
                  {t("preferences.exportChat")}
                </label>
                <div className="flex flex-wrap gap-2.5">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleExportMarkdown}
                    className="flex items-center gap-2 text-xs"
                  >
                    <ArrowDownTrayIcon className="h-4 w-4" />
                    <span>{t("preferences.exportMarkdown")}</span>
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleExportJson}
                    className="flex items-center gap-2 text-xs"
                  >
                    <DocumentTextIcon className="h-4 w-4" />
                    <span>{t("preferences.exportJson")}</span>
                  </Button>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default UserSettingsModal;
