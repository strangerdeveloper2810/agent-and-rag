/**
 * Danh sách hội thoại — /api/conversations.
 *
 * Xoá/đổi tên cập nhật lạc quan vào cache trước rồi mới gọi API, vì sidebar
 * phải phản hồi tức thì. Khác với bản Zustand cũ: nếu request thất bại thì
 * onError trả lại đúng snapshot trước đó, thay vì im lặng để UI lệch với
 * server cho tới lần reload sau.
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import {
  deleteConversation,
  listConversations,
  renameConversation,
  type Conversation,
} from "@/modules/chat/chat.api";
import { STALE_TIME } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";

export type { Conversation };

export const useConversations = () => {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: queryKeys.conversations(),
    queryFn: () => listConversations(),
    staleTime: STALE_TIME.conversations,
  });

  const invalidate = useCallback(
    () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.conversations() }),
    [queryClient],
  );

  /** Sửa cache tại chỗ + trả về snapshot cũ để onError rollback được. */
  const patchCache = useCallback(
    (updater: (list: Conversation[]) => Conversation[]) => {
      const previous = queryClient.getQueryData<Conversation[]>(
        queryKeys.conversations(),
      );
      if (previous) {
        queryClient.setQueryData(queryKeys.conversations(), updater(previous));
      }
      return previous;
    },
    [queryClient],
  );

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteConversation(id),
    onMutate: async (id) => ({
      previous: patchCache((list) => list.filter((c) => c._id !== id)),
    }),
    onError: (_err, _id, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKeys.conversations(), context.previous);
      }
    },
    onSettled: invalidate,
  });

  const renameMutation = useMutation({
    mutationFn: (vars: { id: string; title: string }) =>
      renameConversation(vars.id, vars.title),
    onMutate: async ({ id, title }) => ({
      previous: patchCache((list) =>
        list.map((c) => (c._id === id ? { ...c, title } : c)),
      ),
    }),
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKeys.conversations(), context.previous);
      }
    },
    onSettled: invalidate,
  });

  return {
    conversations: query.data ?? [],
    /** true chỉ ở lần tải đầu — refetch nền không bật spinner của sidebar. */
    loadingConversations: query.isPending,
    reloadConversations: invalidate,
    deleteConv: useCallback(
      (id: string) => deleteMutation.mutateAsync(id).catch(() => undefined),
      [deleteMutation],
    ),
    renameConv: useCallback(
      (id: string, title: string) =>
        renameMutation.mutateAsync({ id, title }).catch(() => undefined),
      [renameMutation],
    ),
  };
};
