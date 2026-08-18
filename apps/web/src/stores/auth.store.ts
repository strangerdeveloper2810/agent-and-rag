/**
 * Auth store — quản lý trạng thái xác thực người dùng.
 *
 * Yêu cầu: cài zustand vào @app/web
 *   pnpm add zustand --filter @app/web
 *
 * State:
 *   - user: thông tin người dùng hiện tại (null nếu chưa đăng nhập)
 *   - isLoading: true khi đang kiểm tra session (init) hoặc đang login/register
 *
 * Actions:
 *   - init(): gọi khi app mount, kiểm tra session qua GET /api/auth/me
 *   - login(email, password): đăng nhập, lưu user vào store
 *   - register(email, password, name): đăng ký — gửi OTP, CHƯA đăng nhập
 *     (user phải verifyEmail() thành công mới có session)
 *   - verifyEmail(email, otp): xác minh OTP, lưu user vào store (đăng nhập lần đầu)
 *   - resendOtp(email): gửi lại OTP (tôn trọng cooldown 2 phút phía server)
 *   - logout(): đăng xuất, xóa user khỏi store
 */

import { create } from "zustand";
import api from "@/lib/http";

// ── Types ──

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  role: string;
  avatar_url?: string | null;
}

interface AuthState {
  user: AuthUser | null;
  isLoading: boolean;
  initialized: boolean;

  init: () => Promise<void>;
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

// ── Store ──

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isLoading: false,
  initialized: false,

  /**
   * Khởi tạo session — gọi khi app mount.
   * Gọi GET /api/auth/me để kiểm tra cookie session còn hợp lệ không.
   */
  init: async () => {
    if (get().initialized) return;
    set({ isLoading: true });
    try {
      const data = await api.get<{ user: AuthUser }>("/api/auth/me");
      set({ user: data.user, isLoading: false, initialized: true });
    } catch {
      set({ user: null, isLoading: false, initialized: true });
    }
  },

  /**
   * Đăng nhập bằng email + password.
   * BFF set httpOnly cookie, store nhận user từ response.
   */
  login: async (email: string, password: string) => {
    set({ isLoading: true });
    try {
      const data = await api.post<{ user: AuthUser }>("/api/auth/login", {
        email,
        password,
      });
      set({ user: data.user, isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },

  /**
   * Đăng ký tài khoản mới — gửi OTP về email, CHƯA đăng nhập.
   * Không set user: phải verifyEmail() thành công mới có session.
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
      set({ user: data.user, isLoading: false });
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
   * Luôn xóa user khỏi store, kể cả khi request lỗi.
   */
  logout: async () => {
    set({ user: null, isLoading: false });
    try {
      await api.post("/api/auth/logout");
    } catch {
      // Ignore
    }
  },
}));

export default useAuthStore;
