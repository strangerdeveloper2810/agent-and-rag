/**
 * QueryClient — tầng cache cho SERVER STATE.
 *
 * Trước đây mọi dữ liệu server đều nằm trong Zustand (auth.store, user.store,
 * ConversationContext). Zustand là store cho CLIENT state: nó không có khái
 * niệm "dữ liệu này còn mới hay đã cũ", nên mỗi component mount lại là gọi
 * API lại — không cách nào dedupe hay tái dùng dữ liệu vừa lấy. Đó là lý do
 * reload 1 trang lại thấy /me, /settings, /suggestions bị gọi trùng.
 *
 * TanStack Query giải quyết đúng chỗ đó: mỗi query có queryKey + staleTime,
 * nhiều component cùng key thì chia sẻ 1 request duy nhất, và trong khoảng
 * staleTime thì mount lại KHÔNG phát request nào.
 *
 * Xem thêm: src/lib/query-keys.ts (danh mục key) và src/hooks/queries/.
 */

import { QueryClient } from "@tanstack/react-query";

/**
 * staleTime cho từng loại dữ liệu — gom về một chỗ để tinh chỉnh, vì đây là
 * đánh đổi giữa "dữ liệu mới" và "số request / tiền LLM".
 *
 * Nguyên tắc chọn: dữ liệu nào chỉ đổi do CHÍNH app này sửa (và mutation đã
 * ghi thẳng vào cache) thì staleTime dài được; dữ liệu nào có thể bị tab khác
 * hoặc backend đổi thì để ngắn.
 */
export const STALE_TIME = {
  /** Session (/api/auth/me): chỉ đổi khi login/logout — mà cả hai đều ghi
   *  thẳng vào cache, nên không cần hỏi lại server thường xuyên. */
  session: 5 * 60_000,

  /** Persona/settings: chỉ đổi qua modal cài đặt của chính user. */
  settings: 5 * 60_000,

  /** MCP servers + custom skills: user tự quản lý trong modal, mutation đã
   *  invalidate sau mỗi thay đổi. 1 phút đủ để mở/đóng modal liên tục mà
   *  không refetch. */
  userResources: 60_000,

  /** Danh sách hội thoại: đổi khi tạo hội thoại mới hoặc đổi tên/xoá — có
   *  invalidate rõ ràng, nhưng vẫn để ngắn vì title được backend sinh ra
   *  không đồng bộ sau tin nhắn đầu tiên. */
  conversations: 30_000,

  /** Lịch sử tin nhắn: append-only. Trong 1 phút quay lại đúng hội thoại đó
   *  thì lấy từ cache (chuyển hội thoại thấy tức thì, không spinner). */
  messages: 60_000,

  /** Gợi ý mở đầu: đây là MỘT LƯỢT GỌI LLM ở agent-go (/suggestions), tốn
   *  token thật và ăn RPM của free tier. Nội dung chỉ là gợi ý chung chung,
   *  không có lý do gì phải mới → cache dài nhất trong hệ. */
  suggestions: 30 * 60_000,
} as const;

/** Đọc HTTP status từ error của http client (ApiError/HttpError đều có .status). */
const httpStatusOf = (error: unknown): number | undefined => {
  const status = (error as { status?: unknown } | null | undefined)?.status;
  return typeof status === "number" ? status : undefined;
};

/**
 * Retry policy: tối đa 1 lần thử lại, và CHỈ khi lỗi có khả năng tự khỏi.
 *
 * Mặc định của TanStack là retry 3 lần cho mọi lỗi — với 4xx thì đó là 3
 * request chắc chắn thất bại (401 lúc chưa đăng nhập, 400 sai payload,
 * 429 đang bị rate limit thì retry còn làm tệ hơn).
 */
export const shouldRetry = (failureCount: number, error: unknown): boolean => {
  if (failureCount >= 1) return false;
  const status = httpStatusOf(error);
  // Không có status → lỗi mạng/timeout, thử lại 1 lần là hợp lý.
  if (status === undefined) return true;
  return status >= 500;
};

export const createQueryClient = (): QueryClient =>
  new QueryClient({
    defaultOptions: {
      queries: {
        // Mặc định 30s: không phải 0 như TanStack để mount lại không tự bắn
        // request. Query nào cần khác thì khai báo staleTime riêng.
        staleTime: STALE_TIME.conversations,
        // Giữ dữ liệu trong bộ nhớ 10 phút sau khi không còn component nào
        // dùng — đủ để đi qua lại giữa các trang mà vẫn còn cache.
        gcTime: 10 * 60_000,
        // TẮT refetch khi focus lại tab. Mặc định của TanStack là true, với
        // /suggestions thì mỗi lần alt-tab về là thêm 1 lượt gọi LLM.
        refetchOnWindowFocus: false,
        retry: shouldRetry,
      },
      mutations: {
        // Mutation không idempotent (tạo hội thoại, tạo MCP server) — retry
        // tự động có thể tạo bản ghi trùng.
        retry: 0,
      },
    },
  });

/**
 * Instance dùng chung cho app. Export ra ngoài React để code không phải
 * component (vd auth.store khi login/logout) vẫn ghi/xoá được cache.
 * Test thì dùng createQueryClient() để mỗi test có cache sạch riêng.
 */
export const queryClient = createQueryClient();
