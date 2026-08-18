/**
 * Auth store — CHỈ còn giữ các hành động (login/register/verify/logout) và cờ
 * isLoading của form.
 *
 * User của session hiện tại KHÔNG còn ở đây nữa: đó là server state, đã
 * chuyển sang TanStack Query (xem src/hooks/queries/useSession.ts). Trước đây
 * store tự giữ `user` + `initialized` + `init()`, nên mỗi guard phải gọi
 * init() trong useEffect — reload trang là 3 lần GET /api/auth/me mà không
 * cách nào dedupe, vì Zustand không biết dữ liệu còn mới hay đã cũ.
 *
 * Các hành động dưới đây ghi thẳng kết quả vào cache ["session"] bằng
 * queryClient (instance dùng chung, xem src/lib/query-client.ts) — endpoint
 * login/verify đã trả về user mới nhất nên không cần gọi thêm /me.
 */

import { create } from "zustand";
import api from "@/lib/http";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type { AuthUser } from "@/hooks/queries/useSession";

export type { AuthUser };

/**
 * Ghi user vào cache session sau khi đăng nhập thành công.
 *
 * clear() TRƯỚC khi ghi: nếu tab này vừa có user khác đăng nhập, cache còn
 * giữ hội thoại/settings của người đó — hiện lại cho người mới là rò rỉ dữ
 * liệu. Sau clear() mới set session để guard không phải gọi lại /me.
 */
const adoptSession = (user: AuthUser) => {
  queryClient.clear();
  queryClient.setQueryData(queryKeys.session(), user);
};

interface AuthState {
  /** true khi đang có một request auth (login/register/...) chạy. */
  isLoading: boolean;

  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  verifyEmail: (email: string, otp: string) => Promise<void>;
  resendOtp: (email: string) => Promise<void>;
  forgotPassword: (email: string) => Promise<void>;
  resetPassword: (
    email: string,
    otp: string,
    newPassword: string,
  ) => Promise<void>;
  logout: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  isLoading: false,

  /**
   * Đăng nhập bằng email + password.
   * BFF set httpOnly cookie, user từ response được ghi vào cache session.
   */
  login: async (email: string, password: string) => {
    set({ isLoading: true });
    try {
      const data = await api.post<{ user: AuthUser }>("/api/auth/login", {
        email,
        password,
      });
      adoptSession(data.user);
      set({ isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  /**
   * Đăng ký tài khoản mới — gửi OTP về email, CHƯA đăng nhập.
   * Không set session: phải verifyEmail() thành công mới có.
   */
  register: async (email: string, password: string, name: string) => {
    set({ isLoading: true });
    try {
      await api.post<{ email: string }>("/api/auth/register", {
        email,
        password,
        name,
      });
      set({ isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  /**
   * Xác minh OTP — thành công thì BFF set httpOnly cookie + trả về user
   * (đăng nhập lần đầu).
   */
  verifyEmail: async (email: string, otp: string) => {
    set({ isLoading: true });
    try {
      const data = await api.post<{ user: AuthUser }>(
        "/api/auth/verify-email",
        { email, otp },
      );
      adoptSession(data.user);
      set({ isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  /** Gửi lại OTP — không đổi isLoading (dùng state riêng ở component page). */
  resendOtp: async (email: string) => {
    await api.post("/api/auth/resend-otp", { email });
  },

  /** Yêu cầu OTP đặt lại mật khẩu */
  forgotPassword: async (email: string) => {
    set({ isLoading: true });
    try {
      await api.post<{ email: string }>("/api/auth/forgot-password", { email });
      set({ isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  /** Đặt lại mật khẩu mới bằng OTP */
  resetPassword: async (email: string, otp: string, newPassword: string) => {
    set({ isLoading: true });
    try {
      await api.post<{ message: string }>("/api/auth/reset-password", {
        email,
        otp,
        newPassword,
      });
      set({ isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  /**
   * Đăng xuất — BFF clear httpOnly cookie.
   *
   * Xoá SẠCH cache (không chỉ session) kể cả khi request lỗi: hội thoại, tin
   * nhắn, settings đều là dữ liệu riêng của user vừa đăng xuất. Ghi
   * session = null ngay sau đó để guard biết "đã kiểm tra, chưa đăng nhập" mà
   * không phải gọi thêm /me chỉ để nhận 401.
   */
  logout: async () => {
    set({ isLoading: false });
    queryClient.clear();
    queryClient.setQueryData(queryKeys.session(), null);
    try {
      await api.post("/api/auth/logout");
    } catch {
      // Ignore
    }
  },
}));

export default useAuthStore;
