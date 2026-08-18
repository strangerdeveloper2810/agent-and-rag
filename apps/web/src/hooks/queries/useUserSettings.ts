/**
 * User settings (persona, avatar agent) — GET/PATCH /api/user/settings.
 *
 * Dữ liệu này được đọc ở 4 chỗ rời rạc (Sidebar, EmptyState, MessageBubble,
 * UserSettingsModal). Trước đây Sidebar fetch một lần rồi nhồi vào Zustand,
 * các chỗ còn lại đọc ké — nghĩa là thứ tự mount quyết định có dữ liệu hay
 * không, và mở modal lại fetch thêm một lần nữa. Với queryKey chung, chỗ nào
 * mount trước thì fetch, còn lại dùng chung cache.
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import api from "@/lib/http";
import { STALE_TIME } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type { AuthUser } from "./useSession";

export interface UserSettings {
  user_id: string;
  persona_preset: "default" | "coder" | "business" | "creative" | "custom";
  formality: "casual" | "neutral" | "formal";
  verbosity: "concise" | "normal" | "detailed";
  humor: "none" | "dry" | "playful";
  custom_instructions: string;
  agent_avatar_url?: string | null;
}

const fetchSettings = async (): Promise<UserSettings> => {
  const res = await api.get<{ settings: UserSettings }>("/api/user/settings");
  return res.settings;
};

/** Đọc settings. Trả về undefined khi chưa tải xong hoặc request lỗi. */
export const useUserSettings = () => {
  const query = useQuery({
    queryKey: queryKeys.settings(),
    queryFn: fetchSettings,
    staleTime: STALE_TIME.settings,
  });
  return { settings: query.data, isPending: query.isPending };
};

/**
 * Chỉ lấy avatar của agent — dùng cho EmptyState/MessageBubble, những chỗ
 * chỉ quan tâm đúng một field. `select` giúp component không re-render khi
 * các field khác của settings đổi.
 */
export const useAgentAvatarUrl = (): string | null | undefined =>
  useQuery({
    queryKey: queryKeys.settings(),
    queryFn: fetchSettings,
    staleTime: STALE_TIME.settings,
    select: (s) => s.agent_avatar_url ?? null,
  }).data;

/** PATCH /api/user/settings — response trả settings mới nên ghi thẳng cache. */
export const useUpdateSettings = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: Partial<UserSettings>) => {
      const res = await api.patch<{ settings: UserSettings }>(
        "/api/user/settings",
        data,
      );
      return res.settings;
    },
    onSuccess: (settings) => {
      queryClient.setQueryData(queryKeys.settings(), settings);
    },
  });
};

/**
 * PATCH /api/user/profile — đổi tên/avatar của USER (không phải của agent),
 * nên kết quả ghi vào cache ["session"], không phải ["user","settings"].
 */
export const useUpdateProfile = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: { name?: string; avatar_url?: string | null }) => {
      const res = await api.patch<{ user: AuthUser }>(
        "/api/user/profile",
        data,
      );
      return res.user;
    },
    onSuccess: (user) => {
      queryClient.setQueryData(queryKeys.session(), user);
    },
  });
};

/** POST /api/user/change-password — không có dữ liệu server nào để cache. */
export const useChangePassword = () =>
  useMutation({
    mutationFn: (vars: { oldPassword: string; newPassword: string }) =>
      api.post("/api/user/change-password", vars),
  });
