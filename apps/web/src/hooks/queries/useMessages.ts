/**
 * Lịch sử tin nhắn — /api/conversations/:id/messages.
 *
 * Khác các query còn lại: ChatPage KHÔNG dùng useQuery ở đây. Lý do là trong
 * lúc stream, mảng messages bị ghi liên tục (mỗi token là một lần nối chuỗi)
 * và còn kèm một Map metadata cục bộ theo từng message. Biến nó thành server
 * state đầy đủ sẽ phải đẩy từng token qua setQueryData — vừa chậm vừa biến
 * cache thành nơi chứa dữ liệu chưa được server xác nhận.
 *
 * Nên ở đây TanStack Query đóng vai trò CACHE ĐỌC:
 *   - fetchMessages(): dùng fetchQuery, tôn trọng staleTime → quay lại một
 *     hội thoại vừa xem là lấy từ cache, không request, không spinner. Hai
 *     lần gọi trùng lúc (StrictMode) chỉ thành một request.
 *   - primeMessages(): sau khi một lượt chat kết thúc, ghi bản mới nhất vào
 *     cache để lần sau quay lại không thấy lịch sử thiếu tin nhắn vừa gửi.
 */

import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { getMessages, type Message } from "@/modules/chat/chat.api";
import { STALE_TIME } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";

export const useMessagesCache = () => {
  const queryClient = useQueryClient();

  return {
    fetchMessages: useCallback(
      (conversationId: string) =>
        queryClient.fetchQuery({
          queryKey: queryKeys.messages(conversationId),
          queryFn: () => getMessages(conversationId),
          staleTime: STALE_TIME.messages,
        }),
      [queryClient],
    ),

    primeMessages: useCallback(
      (conversationId: string, messages: Message[]) =>
        queryClient.setQueryData(queryKeys.messages(conversationId), messages),
      [queryClient],
    ),

    /** Bỏ cache của một hội thoại (vd sau khi xoá nó). */
    dropMessages: useCallback(
      (conversationId: string) =>
        queryClient.removeQueries({
          queryKey: queryKeys.messages(conversationId),
        }),
      [queryClient],
    ),
  };
};
