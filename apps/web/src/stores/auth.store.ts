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
 *   - register(email, password, name): đăng ký, tự động đăng nhập
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
}

interface AuthState {
  user: AuthUser | null;
  isLoading: boolean;

  init: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  logout: () => Promise<void>;
}

// ── Store ──

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: false,

  /**
   * Khởi tạo session — gọi khi app mount.
   * Gọi GET /api/auth/me để kiểm tra cookie session còn hợp lệ không.
   */
  init: async () => {
    set({ isLoading: true });
    try {
      const data = await api.get<{ user: AuthUser }>("/api/auth/me");
      set({ user: data.user, isLoading: false });
    } catch {
      set({ user: null, isLoading: false });
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
   * Đăng ký tài khoản mới — tự động đăng nhập sau khi tạo.
   * BFF set httpOnly cookie + trả về user.
   */
  register: async (email: string, password: string, name: string) => {
    set({ isLoading: true });
    try {
      const data = await api.post<{ user: AuthUser }>("/api/auth/register", {
        email,
        password,
        name,
      });
      set({ user: data.user, isLoading: false });
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
    set({ isLoading: true });
    try {
      await api.post("/api/auth/logout");
    } catch {
      // Bỏ qua lỗi — vẫn xóa user khỏi store
    } finally {
      set({ user: null, isLoading: false });
    }
  },
}));

export default useAuthStore;
