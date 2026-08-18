/**
 * Session — SERVER STATE của user đang đăng nhập (GET /api/auth/me).
 *
 * Trước đây mỗi guard tự gọi `useAuthStore().init()` trong useEffect, cộng
 * thêm một lần gọi ở module scope của main.tsx → reload trang là 3 lần
 * /api/auth/me. Với queryKey ["session"] thì mọi guard/component dùng chung
 * MỘT request, và trong staleTime thì mount lại không phát request nào.
 */

import { useQuery, useQueryClient } from "@tanstack/react-query";
import api from "@/lib/http";
import { STALE_TIME } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  role: string;
  avatar_url?: string | null;
}

/**
 * Lấy user của session hiện tại.
 *
 * QUAN TRỌNG: "chưa đăng nhập" là một KẾT QUẢ HỢP LỆ, không phải lỗi — nên
 * bắt lỗi tại đây và trả null thay vì để query vào trạng thái error. Nếu để
 * throw, TanStack sẽ coi đó là query lỗi và guard phải phân biệt "đang tải"
 * với "lỗi mạng" với "chưa đăng nhập" — trong khi cả hai trường hợp sau đều
 * dẫn tới cùng một hành động: đưa về /login.
 */
export const fetchSession = async (): Promise<AuthUser | null> => {
  try {
    const data = await api.get<{ user: AuthUser }>("/api/auth/me");
    return data.user;
  } catch {
    return null;
  }
};

export interface SessionResult {
  /** User đang đăng nhập, null nếu chưa (hoặc session hết hạn). */
  user: AuthUser | null;
  /** true khi CHƯA có kết quả lần đầu — dùng để hiện spinner ở guard. */
  isPending: boolean;
}

export const useSession = (): SessionResult => {
  const query = useQuery({
    queryKey: queryKeys.session(),
    queryFn: fetchSession,
    staleTime: STALE_TIME.session,
    // fetchSession không bao giờ throw nên retry ở đây là vô nghĩa.
    retry: false,
  });

  return { user: query.data ?? null, isPending: query.isPending };
};

/**
 * Ghi user vào cache session sau khi login/verify/đổi profile thành công.
 *
 * Ghi thẳng vào cache (thay vì invalidate) vì response của các endpoint đó
 * ĐÃ trả về user mới nhất — invalidate chỉ tạo thêm một request /me vô ích.
 */
export const useSetSession = () => {
  const queryClient = useQueryClient();
  return (user: AuthUser | null) =>
    queryClient.setQueryData(queryKeys.session(), user);
};
