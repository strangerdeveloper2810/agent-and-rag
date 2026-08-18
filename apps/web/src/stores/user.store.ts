import { create } from "zustand";
import api from "@/lib/http";
import type { AuthUser } from "./auth.store";

export interface UserSettings {
  user_id: string;
  persona_preset: "default" | "coder" | "business" | "creative" | "custom";
  formality: "casual" | "neutral" | "formal";
  verbosity: "concise" | "normal" | "detailed";
  humor: "none" | "dry" | "playful";
  custom_instructions: string;
  agent_avatar_url?: string | null;
}

export interface McpServer {
  id: string;
  name: string;
  // Spec MCP 2026-07-28 chỉ còn 2 transport: "http" (Streamable HTTP,
  // khuyến nghị) và "sse" (legacy, giữ lại để tương thích ngược).
  transport: "http" | "sse";
  url: string;
  enabled: boolean;
  // API không bao giờ trả lại token đã lưu (chỉ ghi, không đọc) - chỉ báo
  // biết server có auth hay không qua cờ boolean này.
  has_auth: boolean;
  // Số tool đã discovery được từ server (nếu backend đã kết nối thành công).
  tool_count?: number;
}

export interface UserSkill {
  id: string;
  name: string;
  description: string;
  when_to_use: string;
  content: string;
  triggers: string[];
  enabled: boolean;
}

export interface SkillListResult {
  customSkills: UserSkill[];
  disabledBuiltinSkills: string[];
}

interface UserState {
  settings: UserSettings | null;
  isLoading: boolean;
  isSaving: boolean;

  mcpServers: McpServer[];
  skills: UserSkill[];
  disabledBuiltinSkills: string[];
  isLoadingMcp: boolean;
  isLoadingSkills: boolean;

  fetchSettings: () => Promise<UserSettings>;
  updateSettings: (data: Partial<UserSettings>) => Promise<void>;
  updateProfile: (data: {
    name?: string;
    avatar_url?: string | null;
  }) => Promise<AuthUser>;
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>;

  fetchMcpServers: () => Promise<McpServer[]>;
  createMcpServer: (data: {
    name: string;
    transport: "http" | "sse";
    url: string;
    // Optional: chỉ gửi khi user nhập; API sẽ lưu và không bao giờ trả lại.
    auth_token?: string;
  }) => Promise<void>;
  updateMcpServer: (
    id: string,
    data: {
      name?: string;
      transport?: "http" | "sse";
      url?: string;
      enabled?: boolean;
      // Gửi chuỗi rỗng "" nghĩa là xoá token đã lưu (theo contract API).
      auth_token?: string;
    },
  ) => Promise<void>;
  deleteMcpServer: (id: string) => Promise<void>;

  fetchSkills: () => Promise<SkillListResult>;
  createSkill: (data: {
    name: string;
    description?: string;
    when_to_use?: string;
    content: string;
    triggers?: string[];
  }) => Promise<void>;
  updateSkill: (
    id: string,
    data: {
      name?: string;
      description?: string;
      when_to_use?: string;
      content?: string;
      triggers?: string[];
      enabled?: boolean;
    },
  ) => Promise<void>;
  deleteSkill: (id: string) => Promise<void>;
  toggleBuiltinSkill: (name: string, enabled: boolean) => Promise<void>;
}

export const useUserStore = create<UserState>((set) => ({
  settings: null,
  isLoading: false,
  isSaving: false,

  mcpServers: [],
  skills: [],
  disabledBuiltinSkills: [],
  isLoadingMcp: false,
  isLoadingSkills: false,

  fetchSettings: async () => {
    set({ isLoading: true });
    try {
      const res = await api.get<{ settings: UserSettings }>(
        "/api/user/settings",
      );
      set({ settings: res.settings, isLoading: false });
      return res.settings;
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  updateSettings: async (data: Partial<UserSettings>) => {
    set({ isSaving: true });
    try {
      const res = await api.patch<{ settings: UserSettings }>(
        "/api/user/settings",
        data,
      );
      set({ settings: res.settings, isSaving: false });
    } catch (err) {
      set({ isSaving: false });
      throw err;
    }
  },

  updateProfile: async (data) => {
    set({ isSaving: true });
    try {
      const res = await api.patch<{ user: AuthUser }>(
        "/api/user/profile",
        data,
      );
      set({ isSaving: false });
      return res.user;
    } catch (err) {
      set({ isSaving: false });
      throw err;
    }
  },

  changePassword: async (oldPassword: string, newPassword: string) => {
    set({ isSaving: true });
    try {
      await api.post("/api/user/change-password", {
        oldPassword,
        newPassword,
      });
      set({ isSaving: false });
    } catch (err) {
      set({ isSaving: false });
      throw err;
    }
  },

  // ── MCP Servers ──

  fetchMcpServers: async () => {
    set({ isLoadingMcp: true });
    try {
      const res = await api.get<{ servers: McpServer[] }>(
        "/api/user/mcp-servers",
      );
      set({ mcpServers: res.servers, isLoadingMcp: false });
      return res.servers;
    } catch (err) {
      set({ isLoadingMcp: false });
      throw err;
    }
  },

  createMcpServer: async (data) => {
    const res = await api.post<{ server: McpServer }>(
      "/api/user/mcp-servers",
      data,
    );
    set((s) => ({ mcpServers: [...s.mcpServers, res.server] }));
  },

  updateMcpServer: async (id, data) => {
    const res = await api.patch<{ server: McpServer }>(
      `/api/user/mcp-servers/${id}`,
      data,
    );
    set((s) => ({
      mcpServers: s.mcpServers.map((m) => (m.id === id ? res.server : m)),
    }));
  },

  deleteMcpServer: async (id) => {
    await api.del(`/api/user/mcp-servers/${id}`);
    set((s) => ({ mcpServers: s.mcpServers.filter((m) => m.id !== id) }));
  },

  // ── Skills ──

  fetchSkills: async () => {
    set({ isLoadingSkills: true });
    try {
      const res = await api.get<{
        customSkills: UserSkill[];
        disabledBuiltinSkills: string[];
      }>("/api/user/skills");
      set({
        skills: res.customSkills,
        disabledBuiltinSkills: res.disabledBuiltinSkills,
        isLoadingSkills: false,
      });
      return {
        customSkills: res.customSkills,
        disabledBuiltinSkills: res.disabledBuiltinSkills,
      };
    } catch (err) {
      set({ isLoadingSkills: false });
      throw err;
    }
  },

  createSkill: async (data) => {
    const res = await api.post<{ skill: UserSkill }>("/api/user/skills", data);
    set((s) => ({ skills: [...s.skills, res.skill] }));
  },

  updateSkill: async (id, data) => {
    const res = await api.patch<{ skill: UserSkill }>(
      `/api/user/skills/${id}`,
      data,
    );
    set((s) => ({
      skills: s.skills.map((sk) => (sk.id === id ? res.skill : sk)),
    }));
  },

  deleteSkill: async (id) => {
    await api.del(`/api/user/skills/${id}`);
    set((s) => ({ skills: s.skills.filter((sk) => sk.id !== id) }));
  },

  toggleBuiltinSkill: async (name, enabled) => {
    await api.post(`/api/user/skills/${name}/toggle`, { enabled });
    set((s) => ({
      disabledBuiltinSkills: enabled
        ? s.disabledBuiltinSkills.filter((n) => n !== name)
        : s.disabledBuiltinSkills.includes(name)
          ? s.disabledBuiltinSkills
          : [...s.disabledBuiltinSkills, name],
    }));
  },
}));
