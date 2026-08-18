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
}

interface UserState {
  settings: UserSettings | null;
  isLoading: boolean;
  isSaving: boolean;

  fetchSettings: () => Promise<UserSettings>;
  updateSettings: (data: Partial<UserSettings>) => Promise<void>;
  updateProfile: (data: { name?: string; avatar_url?: string | null }) => Promise<AuthUser>;
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>;
}

export const useUserStore = create<UserState>((set) => ({
  settings: null,
  isLoading: false,
  isSaving: false,

  fetchSettings: async () => {
    set({ isLoading: true });
    try {
      const res = await api.get<{ settings: UserSettings }>("/api/user/settings");
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
      const res = await api.patch<{ user: AuthUser }>("/api/user/profile", data);
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
}));
