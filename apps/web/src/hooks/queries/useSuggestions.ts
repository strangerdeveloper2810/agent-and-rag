/**
 * Gợi ý mở đầu hội thoại — GET {agent}/suggestions.
 *
 * ĐÂY LÀ MỘT LƯỢT GỌI LLM ở agent-go, không phải một endpoint đọc DB. Nó tốn
 * token thật và ăn quota RPM của free tier, nên là query đắt nhất trong app
 * và được cache lâu nhất (STALE_TIME.suggestions = 30 phút), tắt
 * refetchOnWindowFocus (đã tắt toàn cục), và không retry khi lỗi.
 *
 * Trả về null khi gọi thất bại hoặc server không có gợi ý nào — component tự
 * lấy pool tĩnh trong file i18n làm phương án dự phòng. Cố tình KHÔNG nhét
 * fallback vào queryFn: fallback phụ thuộc ngôn ngữ UI, nếu nằm trong cache
 * thì đổi ngôn ngữ sẽ hiện gợi ý sai ngôn ngữ cho tới khi cache hết hạn.
 */

import { useQuery } from "@tanstack/react-query";
import { STALE_TIME } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";

const fetchSuggestions = async (): Promise<string[] | null> => {
  const baseUrl = import.meta.env.VITE_AGENT_URL ?? "";
  // cache: "no-store" thay cho tham số `?_t=Date.now()` cũ — vẫn đảm bảo
  // bấm "đổi gợi ý" là gọi thật, nhưng không tạo URL mới mỗi lần render.
  const res = await fetch(`${baseUrl}/suggestions`, { cache: "no-store" });
  if (!res.ok) return null;
  const data = (await res.json()) as { suggestions?: string[] };
  return data.suggestions?.length ? data.suggestions : null;
};

export const useSuggestions = ({
  enabled = true,
}: { enabled?: boolean } = {}) => {
  const query = useQuery({
    queryKey: queryKeys.suggestions(),
    queryFn: fetchSuggestions,
    staleTime: STALE_TIME.suggestions,
    // Gọi lại một lượt LLM chỉ vì lỗi mạng là không đáng — đã có fallback tĩnh.
    retry: false,
    enabled,
  });

  return {
    /** null = không có gợi ý từ agent, component dùng pool tĩnh. */
    suggestions: query.data ?? null,
    isFetching: query.isFetching,
    /** Chỉ gọi từ hành động rõ ràng của user (nút "đổi gợi ý"). */
    refresh: query.refetch,
  };
};
