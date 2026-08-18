/**
 * MCP servers của user — /api/user/mcp-servers.
 *
 * Chỉ fetch khi modal cài đặt mở (`enabled`), và trong staleTime thì đóng/mở
 * lại modal KHÔNG gọi API nữa — trước đây mỗi lần mở là một request.
 *
 * Sau mọi thay đổi thì invalidate (không setQueryData thủ công) vì thứ tự
 * danh sách do server quyết định (ORDER BY created_at) và server còn tính
 * thêm field dẫn xuất như has_auth.
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import api from "@/lib/http";
import { STALE_TIME } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";

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

export interface CreateMcpServerInput {
  name: string;
  transport: "http" | "sse";
  url: string;
  /** Optional: chỉ gửi khi user nhập; API sẽ lưu và không bao giờ trả lại. */
  auth_token?: string;
}

export interface UpdateMcpServerInput {
  name?: string;
  transport?: "http" | "sse";
  url?: string;
  enabled?: boolean;
  /** Gửi chuỗi rỗng "" nghĩa là xoá token đã lưu (theo contract API). */
  auth_token?: string;
}

const fetchMcpServers = async (): Promise<McpServer[]> => {
  const res = await api.get<{ servers: McpServer[] }>("/api/user/mcp-servers");
  return res.servers;
};

export const useMcpServers = ({
  enabled = true,
}: { enabled?: boolean } = {}) => {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: queryKeys.mcpServers(),
    queryFn: fetchMcpServers,
    staleTime: STALE_TIME.userResources,
    enabled,
  });

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.mcpServers() }),
    [queryClient],
  );

  const createMutation = useMutation({
    mutationFn: (data: CreateMcpServerInput) =>
      api.post<{ server: McpServer }>("/api/user/mcp-servers", data),
    onSuccess: invalidate,
  });

  const updateMutation = useMutation({
    mutationFn: (vars: { id: string; data: UpdateMcpServerInput }) =>
      api.patch<{ server: McpServer }>(
        `/api/user/mcp-servers/${vars.id}`,
        vars.data,
      ),
    onSuccess: invalidate,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del(`/api/user/mcp-servers/${id}`),
    onSuccess: invalidate,
  });

  return {
    mcpServers: query.data ?? [],
    isLoadingMcp: query.isPending && enabled,
    createMcpServer: useCallback(
      (data: CreateMcpServerInput) => createMutation.mutateAsync(data),
      [createMutation],
    ),
    updateMcpServer: useCallback(
      (id: string, data: UpdateMcpServerInput) =>
        updateMutation.mutateAsync({ id, data }),
      [updateMutation],
    ),
    deleteMcpServer: useCallback(
      (id: string) => deleteMutation.mutateAsync(id),
      [deleteMutation],
    ),
  };
};
