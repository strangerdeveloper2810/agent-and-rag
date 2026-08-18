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
  ArrowUpTrayIcon,
  TrashIcon,
  PlusIcon,
  BoltIcon,
  PuzzlePieceIcon,
} from "@heroicons/react/24/outline";

import { useSession } from "@/hooks/queries/useSession";
import {
  useChangePassword,
  useUpdateProfile,
  useUpdateSettings,
  useUserSettings,
} from "@/hooks/queries/useUserSettings";
import { useMcpServers } from "@/hooks/queries/useMcpServers";
import { useSkills } from "@/hooks/queries/useSkills";
import { BUILTIN_SKILLS } from "@/stores/builtin-skills";
import { useToast } from "@/design-system/molecules/Toast";
import { useTheme } from "@/hooks/useTheme";
import { persistLocale, type Locale } from "@/i18n/locale";
import { getMessages } from "@/modules/chat/chat.api";
import { uploadImage } from "@/lib/upload";
import ConfirmDialog from "@/design-system/molecules/ConfirmDialog";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";

interface UserSettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  initialTab?:
    "profile" | "persona" | "security" | "preferences" | "mcp" | "skills";
  conversationId?: string;
  conversationMessages?: Array<{
    role: string;
    content: string;
    createdAt?: string;
  }>;
}

const PRESET_OPTIONS = [
  {
    id: "default" as const,
    icon: SparklesIcon,
    accent:
      "from-amber-500/20 to-orange-500/20 border-amber-500/30 text-amber-500",
  },
  {
    id: "coder" as const,
    icon: CommandLineIcon,
    accent:
      "from-emerald-500/20 to-teal-500/20 border-emerald-500/30 text-emerald-500",
  },
  {
    id: "business" as const,
    icon: BriefcaseIcon,
    accent:
      "from-blue-500/20 to-indigo-500/20 border-blue-500/30 text-blue-500",
  },
  {
    id: "creative" as const,
    icon: LightBulbIcon,
    accent:
      "from-purple-500/20 to-pink-500/20 border-purple-500/30 text-purple-500",
  },
  {
    id: "custom" as const,
    icon: WrenchScrewdriverIcon,
    accent:
      "from-slate-500/20 to-zinc-500/20 border-slate-500/30 text-slate-400",
  },
];

// Số builtin skill hiện mặc định trước khi bấm "Xem tất cả" - tránh tab Skills
// dài vô tận khi có tới 32 skill (xem BUILTIN_SKILLS).
const BUILTIN_VISIBLE_COUNT = 8;

/**
 * ToggleSwitch — công tắc bật/tắt dùng chung cho MCP server & skill.
 * Design-system hiện chưa có component Switch riêng nên dựng tạm bằng button + span,
 * bám theo token màu bg-primary/bg-muted để tự đồng bộ theme sáng/tối.
 */
const ToggleSwitch: React.FC<{
  checked: boolean;
  onChange: () => void;
  label: string;
}> = ({ checked, onChange, label }) => (
  <button
    type="button"
    role="switch"
    aria-checked={checked}
    aria-label={label}
    onClick={onChange}
    className={`inline-flex h-6 w-11 shrink-0 items-center rounded-full border transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-2 focus-visible:ring-offset-card ${
      checked ? "bg-primary border-primary" : "bg-muted border-border"
    }`}
  >
    {/* Knob là flex child, KHÔNG absolute: absolute mà thiếu neo left/inset thì
        vị trí ngang phụ thuộc static position của button rỗng nên núm không hiện
        (đúng lỗi đã report). translate-x dịch trong khung 44px: 2px khi tắt,
        22px khi bật (44 - 20 - 2). */}
    <span
      aria-hidden="true"
      className={`pointer-events-none block h-5 w-5 rounded-full bg-white shadow-sm ring-1 ring-black/5 transition-transform duration-200 ${
        checked ? "translate-x-[22px]" : "translate-x-[2px]"
      }`}
    />
  </button>
);

export const UserSettingsModal: React.FC<UserSettingsModalProps> = ({
  isOpen,
  onClose,
  initialTab = "persona",
  conversationId,
  conversationMessages = [],
}) => {
  const { t, i18n } = useTranslation("settings");
  const { user } = useSession();
  const { settings } = useUserSettings();

  const updateSettingsMutation = useUpdateSettings();
  const updateProfileMutation = useUpdateProfile();
  const changePasswordMutation = useChangePassword();
  const updateSettings = updateSettingsMutation.mutateAsync;
  const updateProfile = updateProfileMutation.mutateAsync;
  const changePassword = (oldPassword: string, newPassword: string) =>
    changePasswordMutation.mutateAsync({ oldPassword, newPassword });
  // Ba form (profile / persona / mật khẩu) dùng chung một cờ "đang lưu" để
  // disable nút submit, đúng như isSaving của store cũ.
  const isSaving =
    updateSettingsMutation.isPending ||
    updateProfileMutation.isPending ||
    changePasswordMutation.isPending;

  // enabled: isOpen — modal luôn được mount (nó tự return null khi đóng), nên
  // nếu không chặn thì mọi user vừa vào app là đã fetch MCP + skills dù chưa
  // mở cài đặt. Trong staleTime thì đóng/mở lại không gọi API nữa.
  const {
    mcpServers,
    isLoadingMcp,
    createMcpServer,
    updateMcpServer,
    deleteMcpServer,
  } = useMcpServers({ enabled: isOpen });
  const {
    skills,
    disabledBuiltinSkills,
    isLoadingSkills,
    createSkill,
    updateSkill,
    deleteSkill,
    toggleBuiltinSkill,
  } = useSkills({ enabled: isOpen });
  const toast = useToast();
  const { theme, setTheme } = useTheme();

  const locale = (i18n.language === "en" ? "en" : "vi") as Locale;
  const changeLocale = (newLoc: Locale) => {
    i18n.changeLanguage(newLoc);
    persistLocale(newLoc);
  };

  const [activeTab, setActiveTab] = useState<
    "profile" | "persona" | "security" | "preferences" | "mcp" | "skills"
  >(initialTab);

  // Avatar upload state
  const [avatarUrl, setAvatarUrl] = useState<string | null>(
    user?.avatar_url ?? null,
  );
  const [agentAvatarUrl, setAgentAvatarUrl] = useState<string | null>(null);
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false);
  const [isUploadingAgentAvatar, setIsUploadingAgentAvatar] = useState(false);

  // MCP form state
  const [mcpName, setMcpName] = useState("");
  // Transport mặc định "http" (Streamable HTTP) theo khuyến nghị của spec MCP
  // 2026-07-28; "sse" chỉ còn giữ cho server legacy chưa nâng cấp transport.
  const [mcpTransport, setMcpTransport] = useState<"http" | "sse">("http");
  const [mcpUrl, setMcpUrl] = useState("");
  // Token nhập qua input password - write-only, API không bao giờ trả lại token đã lưu.
  const [mcpAuthToken, setMcpAuthToken] = useState("");

  // Custom skill form state
  const [skillName, setSkillName] = useState("");
  const [skillDescription, setSkillDescription] = useState("");
  const [skillWhenToUse, setSkillWhenToUse] = useState("");
  const [skillContent, setSkillContent] = useState("");
  const [skillTriggers, setSkillTriggers] = useState("");
  // Form "Tạo skill mới" mặc định đóng để tab không dài thêm nữa (32 builtin skill
  // đã chiếm nhiều chỗ) - bấm nút "+ Tạo skill" mới mở.
  const [isCreateSkillOpen, setIsCreateSkillOpen] = useState(false);
  // Ô tìm kiếm để filter 32 builtin skill theo tên thay vì bắt cuộn qua toàn bộ.
  const [skillSearch, setSkillSearch] = useState("");
  // Mặc định chỉ hiện BUILTIN_VISIBLE_COUNT skill đầu; bấm "Xem tất cả" mới hiện hết.
  const [showAllBuiltinSkills, setShowAllBuiltinSkills] = useState(false);

  // Xác nhận xoá (dùng chung ConfirmDialog) - lưu cả id lẫn name để hiện
  // message mà không cần tra lại list (list có thể đã đổi sau khi xoá xong)
  const [pendingDeleteMcp, setPendingDeleteMcp] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [pendingDeleteSkill, setPendingDeleteSkill] = useState<{
    id: string;
    name: string;
  } | null>(null);

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
      setAvatarUrl(user?.avatar_url ?? null);
    }
  }, [isOpen, initialTab, user]);

  // Nạp form persona từ settings. Trước đây chỗ này gọi fetchSettings() rồi
  // seed trong .then() — tức là mỗi lần mở modal là một request. Giờ settings
  // đến từ cache dùng chung, effect chỉ chịu trách nhiệm copy vào form state
  // (form là client state, phải cho user sửa được nên không đọc trực tiếp).
  useEffect(() => {
    if (!isOpen || !settings) return;
    setPersonaPreset(settings.persona_preset || "default");
    setFormality(settings.formality || "neutral");
    setVerbosity(settings.verbosity || "normal");
    setCustomInstructions(settings.custom_instructions || "");
    setAgentAvatarUrl(settings.agent_avatar_url ?? null);
  }, [isOpen, settings]);

  if (!isOpen) return null;

  // Lọc builtin skill theo tên (không dấu-nhạy phân biệt hoa/thường) để filter
  // hoạt động tốt cho cả tên tiếng Anh (vd "code-review").
  const normalizedSkillSearch = skillSearch.trim().toLowerCase();
  const filteredBuiltinSkills = normalizedSkillSearch
    ? BUILTIN_SKILLS.filter((skill) =>
        skill.name.toLowerCase().includes(normalizedSkillSearch),
      )
    : BUILTIN_SKILLS;
  // Khi đang tìm kiếm thì hiện hết kết quả filter (thường đã ít) - không áp dụng
  // giới hạn "Xem tất cả" nữa vì sẽ ẩn mất kết quả người dùng đang tìm.
  const displayedBuiltinSkills =
    showAllBuiltinSkills || normalizedSkillSearch
      ? filteredBuiltinSkills
      : filteredBuiltinSkills.slice(0, BUILTIN_VISIBLE_COUNT);
  const canToggleShowAllBuiltinSkills =
    !normalizedSkillSearch &&
    filteredBuiltinSkills.length > BUILTIN_VISIBLE_COUNT;

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

  const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setIsUploadingAvatar(true);
    try {
      const url = await uploadImage(file);
      await updateProfile({ avatar_url: url });
      setAvatarUrl(url);
      toast.success(t("profile.avatarSuccess"));
    } catch (err: any) {
      toast.error(err?.message || "Lỗi tải ảnh đại diện");
    } finally {
      setIsUploadingAvatar(false);
      e.target.value = "";
    }
  };

  const handleAgentAvatarUpload = async (
    e: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setIsUploadingAgentAvatar(true);
    try {
      const url = await uploadImage(file);
      await updateSettings({ agent_avatar_url: url });
      setAgentAvatarUrl(url);
      toast.success(t("persona.avatarSuccess"));
    } catch (err: any) {
      toast.error(err?.message || "Lỗi tải ảnh agent");
    } finally {
      setIsUploadingAgentAvatar(false);
      e.target.value = "";
    }
  };

  const handleAddMcpServer = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!mcpName.trim() || !mcpUrl.trim()) return;
    try {
      await createMcpServer({
        name: mcpName.trim(),
        transport: mcpTransport,
        url: mcpUrl.trim(),
        // Chỉ gửi auth_token khi user thực sự nhập - contract chỉ coi chuỗi
        // rỗng là "xoá token" cho PATCH, không áp dụng khi tạo mới (POST).
        ...(mcpAuthToken.trim() ? { auth_token: mcpAuthToken.trim() } : {}),
      });
      setMcpName("");
      setMcpUrl("");
      setMcpTransport("http");
      setMcpAuthToken("");
      toast.success(t("mcp.addSuccess"));
    } catch (err: any) {
      toast.error(err?.message || "Lỗi thêm MCP server");
    }
  };

  const handleToggleMcpServer = async (id: string, enabled: boolean) => {
    try {
      await updateMcpServer(id, { enabled });
    } catch (err: any) {
      toast.error(err?.message || "Lỗi cập nhật MCP server");
    }
  };

  const handleDeleteMcpServer = async (id: string) => {
    try {
      await deleteMcpServer(id);
    } catch (err: any) {
      toast.error(err?.message || "Lỗi xoá MCP server");
    }
  };

  const handleAddSkill = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!skillName.trim() || !skillContent.trim()) return;
    try {
      await createSkill({
        name: skillName.trim(),
        description: skillDescription.trim(),
        when_to_use: skillWhenToUse.trim(),
        content: skillContent.trim(),
        triggers: skillTriggers
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean),
      });
      setSkillName("");
      setSkillDescription("");
      setSkillWhenToUse("");
      setSkillContent("");
      setSkillTriggers("");
      // Đóng form lại sau khi tạo thành công - đúng luồng "bấm mới mở" của form thu gọn.
      setIsCreateSkillOpen(false);
      toast.success(t("skills.addSuccess"));
    } catch (err: any) {
      toast.error(err?.message || "Lỗi thêm skill");
    }
  };

  const handleToggleBuiltinSkill = async (name: string, enabled: boolean) => {
    try {
      await toggleBuiltinSkill(name, enabled);
    } catch (err: any) {
      toast.error(err?.message || "Lỗi cập nhật skill");
    }
  };

  const handleToggleCustomSkill = async (id: string, enabled: boolean) => {
    try {
      await updateSkill(id, { enabled });
    } catch (err: any) {
      toast.error(err?.message || "Lỗi cập nhật skill");
    }
  };

  const handleDeleteSkill = async (id: string) => {
    try {
      await deleteSkill(id);
    } catch (err: any) {
      toast.error(err?.message || "Lỗi xoá skill");
    }
  };

  // Xác nhận xoá MCP server qua ConfirmDialog rồi mới gọi handler xoá thật
  const confirmDeleteMcp = async () => {
    if (!pendingDeleteMcp) return;
    await handleDeleteMcpServer(pendingDeleteMcp.id);
    setPendingDeleteMcp(null);
  };

  // Xác nhận xoá custom skill qua ConfirmDialog rồi mới gọi handler xoá thật
  const confirmDeleteSkill = async () => {
    if (!pendingDeleteSkill) return;
    await handleDeleteSkill(pendingDeleteSkill.id);
    setPendingDeleteSkill(null);
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
            <p className="text-xs text-muted-foreground">
              {t("modal.subtitle")}
            </p>
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
          <button
            onClick={() => setActiveTab("mcp")}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition shrink-0 ${
              activeTab === "mcp"
                ? "bg-primary/10 text-primary"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            <BoltIcon className="h-4 w-4" />
            <span>{t("modal.tabs.mcp")}</span>
          </button>
          <button
            onClick={() => setActiveTab("skills")}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition shrink-0 ${
              activeTab === "skills"
                ? "bg-primary/10 text-primary"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            <PuzzlePieceIcon className="h-4 w-4" />
            <span>{t("modal.tabs.skills")}</span>
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

              {/* Avatar Agent */}
              <div className="flex items-center gap-4 p-4 rounded-xl bg-muted/20 border border-border">
                <div className="relative shrink-0">
                  <Avatar className="h-16 w-16">
                    {agentAvatarUrl ? (
                      <AvatarImage
                        src={agentAvatarUrl}
                        alt={t("persona.avatarTitle")}
                      />
                    ) : (
                      <AvatarFallback className="bg-gradient-to-br from-amber-500 to-orange-600 text-white">
                        <SparklesIcon className="h-6 w-6" />
                      </AvatarFallback>
                    )}
                  </Avatar>
                  <label
                    htmlFor="agent-avatar-upload"
                    title={t("persona.avatarUploadLabel")}
                    className={`absolute -bottom-1 -right-1 flex h-6 w-6 items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-sm transition hover:text-primary hover:border-primary ${
                      isUploadingAgentAvatar
                        ? "opacity-60 pointer-events-none"
                        : "cursor-pointer"
                    }`}
                  >
                    {isUploadingAgentAvatar ? (
                      <span className="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent" />
                    ) : (
                      <ArrowUpTrayIcon className="h-3 w-3" />
                    )}
                    <input
                      id="agent-avatar-upload"
                      type="file"
                      accept="image/*"
                      className="hidden"
                      disabled={isUploadingAgentAvatar}
                      onChange={handleAgentAvatarUpload}
                      aria-label={t("persona.avatarUploadLabel")}
                    />
                  </label>
                </div>
                <div>
                  <h4 className="text-sm font-bold text-foreground">
                    {t("persona.avatarTitle")}
                  </h4>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    {t("persona.avatarDesc")}
                  </p>
                  <p className="text-[11px] text-muted-foreground mt-1">
                    {isUploadingAgentAvatar
                      ? t("persona.avatarUploading")
                      : t("persona.avatarHint")}
                  </p>
                </div>
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
                <div className="relative shrink-0">
                  <Avatar className="h-16 w-16">
                    {avatarUrl ? (
                      <AvatarImage
                        src={avatarUrl}
                        alt={user?.name || t("profile.title")}
                      />
                    ) : (
                      <AvatarFallback className="bg-gradient-to-br from-indigo-500 to-purple-600 text-white font-bold text-lg">
                        {userInitials}
                      </AvatarFallback>
                    )}
                  </Avatar>
                  <label
                    htmlFor="profile-avatar-upload"
                    title={t("profile.avatarUploadLabel")}
                    className={`absolute -bottom-1 -right-1 flex h-6 w-6 items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-sm transition hover:text-primary hover:border-primary ${
                      isUploadingAvatar
                        ? "opacity-60 pointer-events-none"
                        : "cursor-pointer"
                    }`}
                  >
                    {isUploadingAvatar ? (
                      <span className="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent" />
                    ) : (
                      <ArrowUpTrayIcon className="h-3 w-3" />
                    )}
                    <input
                      id="profile-avatar-upload"
                      type="file"
                      accept="image/*"
                      className="hidden"
                      disabled={isUploadingAvatar}
                      onChange={handleAvatarUpload}
                      aria-label={t("profile.avatarUploadLabel")}
                    />
                  </label>
                </div>
                <div>
                  <h4 className="text-sm font-bold text-foreground">
                    {user?.name || "Người dùng"}
                  </h4>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    {user?.email}
                  </p>
                  <p className="text-[11px] text-muted-foreground mt-1">
                    {isUploadingAvatar
                      ? t("profile.avatarUploading")
                      : t("profile.avatarHint")}
                  </p>
                  <Badge
                    variant="outline"
                    className="mt-2 text-[10px] uppercase"
                  >
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
                  {isSaving
                    ? t("modal.saving")
                    : t("security.changePasswordButton")}
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
                    <span className="text-xs">
                      {t("preferences.theme.dark")}
                    </span>
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
                    <span className="text-xs">
                      {t("preferences.theme.light")}
                    </span>
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

          {/* TAB 5: MCP SERVERS */}
          {activeTab === "mcp" && (
            <div className="space-y-6">
              <div>
                <h3 className="text-sm font-bold text-foreground">
                  {t("mcp.title")}
                </h3>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {t("mcp.desc")}
                </p>
              </div>

              {/* Form thêm MCP server mới */}
              <form
                onSubmit={handleAddMcpServer}
                className="space-y-3 p-4 rounded-xl bg-muted/20 border border-border"
              >
                <h4 className="text-xs font-bold text-foreground">
                  {t("mcp.addTitle")}
                </h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[11px] font-semibold text-muted-foreground mb-1">
                      {t("mcp.nameLabel")}
                    </label>
                    <Input
                      type="text"
                      value={mcpName}
                      onChange={(e) => setMcpName(e.target.value)}
                      placeholder={t("mcp.namePlaceholder")}
                    />
                  </div>
                  <div>
                    <label
                      htmlFor="mcp-transport-select"
                      className="block text-[11px] font-semibold text-muted-foreground mb-1"
                    >
                      {t("mcp.transportLabel")}
                    </label>
                    {/* Native <select> vì design-system chưa có component Select riêng -
                        chỉ 2 lựa chọn nên select thường là đủ, không cần thêm dependency. */}
                    <select
                      id="mcp-transport-select"
                      value={mcpTransport}
                      onChange={(e) =>
                        setMcpTransport(e.target.value as "http" | "sse")
                      }
                      className="flex h-10 w-full rounded-xl border border-input bg-background/60 px-3.5 py-2 text-sm text-foreground ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 backdrop-blur-md transition duration-150"
                    >
                      <option value="http">{t("mcp.transport.http")}</option>
                      <option value="sse">{t("mcp.transport.sse")}</option>
                    </select>
                  </div>
                </div>
                <div>
                  <label className="block text-[11px] font-semibold text-muted-foreground mb-1">
                    {t("mcp.urlLabel")}
                  </label>
                  <Input
                    type="text"
                    value={mcpUrl}
                    onChange={(e) => setMcpUrl(e.target.value)}
                    placeholder={t("mcp.urlPlaceholder")}
                  />
                  <p className="text-[11px] text-muted-foreground mt-1">
                    {t("mcp.urlHint")}
                  </p>
                </div>
                <div>
                  <label className="block text-[11px] font-semibold text-muted-foreground mb-1">
                    {t("mcp.authTokenLabel")}
                  </label>
                  <Input
                    type="password"
                    value={mcpAuthToken}
                    onChange={(e) => setMcpAuthToken(e.target.value)}
                    placeholder={t("mcp.authTokenPlaceholder")}
                    autoComplete="off"
                  />
                  <p className="text-[11px] text-muted-foreground mt-1">
                    {t("mcp.authTokenHint")}
                  </p>
                </div>
                <div className="flex justify-end">
                  <Button
                    type="submit"
                    variant="gradient"
                    size="sm"
                    disabled={!mcpName.trim() || !mcpUrl.trim()}
                    className="flex items-center gap-1.5"
                  >
                    <PlusIcon className="h-4 w-4" />
                    <span>{t("mcp.addButton")}</span>
                  </Button>
                </div>
              </form>

              {/* Danh sách MCP servers */}
              <div>
                <h4 className="text-xs font-bold text-foreground mb-2">
                  {t("mcp.listTitle")}
                </h4>
                {isLoadingMcp ? (
                  <div
                    className="space-y-2"
                    aria-live="polite"
                    aria-busy="true"
                  >
                    <Skeleton className="h-14 w-full" />
                    <Skeleton className="h-14 w-full" />
                  </div>
                ) : mcpServers.length === 0 ? (
                  <div className="py-8 text-center rounded-xl border border-dashed border-border text-xs text-muted-foreground">
                    {t("mcp.empty")}
                  </div>
                ) : (
                  <div className="space-y-2">
                    {mcpServers.map((server) => (
                      <div
                        key={server.id}
                        className={`flex items-center gap-3 p-3 rounded-xl bg-card border border-border transition ${
                          server.enabled ? "" : "opacity-60"
                        }`}
                      >
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="text-xs font-bold text-foreground truncate">
                              {server.name}
                            </span>
                            <Badge
                              variant="accent"
                              className="text-[10px] uppercase shrink-0"
                            >
                              {server.transport === "http"
                                ? t("mcp.transportBadge.http")
                                : t("mcp.transportBadge.sse")}
                            </Badge>
                            {server.has_auth && (
                              <Badge
                                variant="success"
                                className="text-[10px] uppercase shrink-0"
                              >
                                {t("mcp.hasAuthBadge")}
                              </Badge>
                            )}
                            {/* Không hiện badge riêng khi enabled - row bị làm mờ
                                (opacity-60) khi tắt là đủ để phân biệt trạng thái. */}
                            {!server.enabled && (
                              <Badge
                                variant="outline"
                                className="text-[10px] uppercase shrink-0"
                              >
                                {t("mcp.statusDisabled")}
                              </Badge>
                            )}
                          </div>
                          <p className="text-[11px] text-muted-foreground truncate mt-0.5">
                            {server.url}
                          </p>
                          {typeof server.tool_count === "number" && (
                            <p className="text-[11px] text-muted-foreground mt-0.5">
                              {t("mcp.toolCount", { count: server.tool_count })}
                            </p>
                          )}
                        </div>
                        <ToggleSwitch
                          checked={server.enabled}
                          onChange={() =>
                            handleToggleMcpServer(server.id, !server.enabled)
                          }
                          label={
                            server.enabled
                              ? t("mcp.disableAria", { name: server.name })
                              : t("mcp.enableAria", { name: server.name })
                          }
                        />
                        <button
                          type="button"
                          onClick={() =>
                            setPendingDeleteMcp({
                              id: server.id,
                              name: server.name,
                            })
                          }
                          aria-label={t("mcp.deleteAria", {
                            name: server.name,
                          })}
                          className="p-1.5 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition shrink-0"
                        >
                          <TrashIcon className="h-4 w-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}

          {/* TAB 6: SKILLS */}
          {activeTab === "skills" && (
            <div className="space-y-6">
              <div>
                <h3 className="text-sm font-bold text-foreground">
                  {t("skills.title")}
                </h3>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {t("skills.desc")}
                </p>
              </div>

              {/* Builtin skills */}
              <div>
                <div className="flex items-center justify-between gap-3 mb-2">
                  <h4 className="text-xs font-bold text-foreground">
                    {t("skills.builtinTitle")}
                  </h4>
                  <span className="text-[11px] text-muted-foreground shrink-0">
                    {t("skills.builtinCount", { count: BUILTIN_SKILLS.length })}
                  </span>
                </div>

                {/* Ô tìm kiếm/filter theo tên - tránh bắt cuộn qua 32 skill có sẵn */}
                <Input
                  type="text"
                  value={skillSearch}
                  onChange={(e) => setSkillSearch(e.target.value)}
                  placeholder={t("skills.searchPlaceholder")}
                  className="mb-2"
                />

                {isLoadingSkills ? (
                  <div
                    className="space-y-2"
                    aria-live="polite"
                    aria-busy="true"
                  >
                    <Skeleton className="h-12 w-full" />
                    <Skeleton className="h-12 w-full" />
                    <Skeleton className="h-12 w-full" />
                  </div>
                ) : filteredBuiltinSkills.length === 0 ? (
                  <div className="py-6 text-center rounded-xl border border-dashed border-border text-xs text-muted-foreground">
                    {t("skills.noResults")}
                  </div>
                ) : (
                  <>
                    {/* Bỏ hộp scroll cụt cao cố định (max-h-64 overflow-y-auto): modal
                        body đã tự scroll ở ngoài (flex-1 overflow-y-auto), lồng thêm
                        1 lớp scroll bên trong khiến item bị cắt ngang giữa chừng mà
                        không có dấu hiệu còn nội dung bên dưới - đây chính là bug đã
                        report. Thay bằng show-8-đầu + nút "Xem tất cả". */}
                    <div className="space-y-2">
                      {displayedBuiltinSkills.map((skill) => {
                        const enabled = !disabledBuiltinSkills.includes(
                          skill.name,
                        );
                        return (
                          <div
                            key={skill.name}
                            className={`flex items-center gap-3 p-3 rounded-xl bg-card border border-border transition ${
                              enabled ? "" : "opacity-60"
                            }`}
                          >
                            <div className="min-w-0 flex-1">
                              <div className="flex items-center gap-2">
                                <span className="text-xs font-bold text-foreground truncate">
                                  {skill.name}
                                </span>
                                {!enabled && (
                                  <Badge
                                    variant="outline"
                                    className="text-[10px] uppercase shrink-0"
                                  >
                                    {t("skills.disabledBadge")}
                                  </Badge>
                                )}
                              </div>
                              <p className="text-[11px] text-muted-foreground line-clamp-2 mt-0.5">
                                {skill.description}
                              </p>
                            </div>
                            <ToggleSwitch
                              checked={enabled}
                              onChange={() =>
                                handleToggleBuiltinSkill(skill.name, !enabled)
                              }
                              label={
                                enabled
                                  ? t("skills.disableAria", {
                                      name: skill.name,
                                    })
                                  : t("skills.enableAria", { name: skill.name })
                              }
                            />
                          </div>
                        );
                      })}
                    </div>
                    {canToggleShowAllBuiltinSkills && (
                      <div className="flex justify-center mt-2">
                        <button
                          type="button"
                          onClick={() => setShowAllBuiltinSkills((v) => !v)}
                          className="text-[11px] font-semibold text-primary hover:underline"
                        >
                          {showAllBuiltinSkills
                            ? t("skills.showLess")
                            : t("skills.showAll")}
                        </button>
                      </div>
                    )}
                  </>
                )}
              </div>

              {/* Custom skills */}
              <div className="pt-4 border-t border-border">
                <h4 className="text-xs font-bold text-foreground mb-2">
                  {t("skills.customTitle")}
                </h4>
                {isLoadingSkills ? (
                  <div
                    className="space-y-2"
                    aria-live="polite"
                    aria-busy="true"
                  >
                    <Skeleton className="h-12 w-full" />
                  </div>
                ) : skills.length === 0 ? (
                  <div className="py-6 text-center rounded-xl border border-dashed border-border text-xs text-muted-foreground">
                    {t("skills.customEmpty")}
                  </div>
                ) : (
                  <div className="space-y-2">
                    {skills.map((skill) => (
                      <div
                        key={skill.id}
                        className={`flex items-center gap-3 p-3 rounded-xl bg-card border border-border transition ${
                          skill.enabled ? "" : "opacity-60"
                        }`}
                      >
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className="text-xs font-bold text-foreground truncate">
                              {skill.name}
                            </span>
                            {!skill.enabled && (
                              <Badge
                                variant="outline"
                                className="text-[10px] uppercase shrink-0"
                              >
                                {t("skills.disabledBadge")}
                              </Badge>
                            )}
                          </div>
                          {skill.description && (
                            <p className="text-[11px] text-muted-foreground line-clamp-2 mt-0.5">
                              {skill.description}
                            </p>
                          )}
                        </div>
                        <ToggleSwitch
                          checked={skill.enabled}
                          onChange={() =>
                            handleToggleCustomSkill(skill.id, !skill.enabled)
                          }
                          label={
                            skill.enabled
                              ? t("skills.disableAria", { name: skill.name })
                              : t("skills.enableAria", { name: skill.name })
                          }
                        />
                        <button
                          type="button"
                          onClick={() =>
                            setPendingDeleteSkill({
                              id: skill.id,
                              name: skill.name,
                            })
                          }
                          aria-label={t("skills.deleteAria", {
                            name: skill.name,
                          })}
                          className="p-1.5 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition shrink-0"
                        >
                          <TrashIcon className="h-4 w-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Form thêm custom skill mới - thu gọn mặc định, bấm "+ Tạo skill"
                  mới mở, tránh tab dài vô tận khi cộng dồn với 32 builtin skill. */}
              {!isCreateSkillOpen ? (
                <div className="flex justify-center pt-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setIsCreateSkillOpen(true)}
                    className="flex items-center gap-1.5"
                  >
                    <PlusIcon className="h-4 w-4" />
                    <span>{t("skills.createToggleOpen")}</span>
                  </Button>
                </div>
              ) : (
                <form
                  onSubmit={handleAddSkill}
                  className="space-y-3 p-4 rounded-xl bg-muted/20 border border-border"
                >
                  <div className="flex items-center justify-between">
                    <h4 className="text-xs font-bold text-foreground">
                      {t("skills.addTitle")}
                    </h4>
                    <button
                      type="button"
                      onClick={() => setIsCreateSkillOpen(false)}
                      aria-label={t("skills.closeFormAria")}
                      className="p-1 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition"
                    >
                      <XMarkIcon className="h-4 w-4" />
                    </button>
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-muted-foreground mb-1">
                      {t("skills.nameLabel")}
                    </label>
                    <Input
                      type="text"
                      value={skillName}
                      onChange={(e) => setSkillName(e.target.value)}
                      placeholder={t("skills.namePlaceholder")}
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-muted-foreground mb-1">
                      {t("skills.descriptionLabel")}
                    </label>
                    <Input
                      type="text"
                      value={skillDescription}
                      onChange={(e) => setSkillDescription(e.target.value)}
                      placeholder={t("skills.descriptionPlaceholder")}
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-muted-foreground mb-1">
                      {t("skills.whenToUseLabel")}
                    </label>
                    <Input
                      type="text"
                      value={skillWhenToUse}
                      onChange={(e) => setSkillWhenToUse(e.target.value)}
                      placeholder={t("skills.whenToUsePlaceholder")}
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-muted-foreground mb-1">
                      {t("skills.contentLabel")}
                    </label>
                    <textarea
                      rows={4}
                      value={skillContent}
                      onChange={(e) => setSkillContent(e.target.value)}
                      placeholder={t("skills.contentPlaceholder")}
                      className="w-full rounded-xl bg-background border border-border p-3 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary transition resize-none"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-muted-foreground mb-1">
                      {t("skills.triggersLabel")}
                    </label>
                    <Input
                      type="text"
                      value={skillTriggers}
                      onChange={(e) => setSkillTriggers(e.target.value)}
                      placeholder={t("skills.triggersPlaceholder")}
                    />
                    <p className="text-[11px] text-muted-foreground mt-1">
                      {t("skills.triggersHint")}
                    </p>
                  </div>
                  <div className="flex justify-end">
                    <Button
                      type="submit"
                      variant="gradient"
                      size="sm"
                      disabled={!skillName.trim() || !skillContent.trim()}
                      className="flex items-center gap-1.5"
                    >
                      <PlusIcon className="h-4 w-4" />
                      <span>{t("skills.addButton")}</span>
                    </Button>
                  </div>
                </form>
              )}
            </div>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={pendingDeleteMcp !== null}
        title={t("mcp.deleteConfirmTitle")}
        message={
          pendingDeleteMcp
            ? t("mcp.deleteConfirmMessage", { name: pendingDeleteMcp.name })
            : undefined
        }
        confirmLabel={t("mcp.deleteConfirmButton")}
        danger
        onConfirm={confirmDeleteMcp}
        onCancel={() => setPendingDeleteMcp(null)}
      />

      <ConfirmDialog
        open={pendingDeleteSkill !== null}
        title={t("skills.deleteConfirmTitle")}
        message={
          pendingDeleteSkill
            ? t("skills.deleteConfirmMessage", {
                name: pendingDeleteSkill.name,
              })
            : undefined
        }
        confirmLabel={t("skills.deleteConfirmButton")}
        danger
        onConfirm={confirmDeleteSkill}
        onCancel={() => setPendingDeleteSkill(null)}
      />
    </div>
  );
};

export default UserSettingsModal;
