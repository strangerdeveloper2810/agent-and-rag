/**
 * HTTP client với auto-refresh token.
 *
 * Dùng native fetch API (không axios). Tự động gọi /api/auth/refresh khi
 * nhận 401, sau đó retry request gốc đúng 1 lần. Gửi kèm cookie
 * (credentials: "include") để BFF set/clear httpOnly cookies.
 *
 * Các helper export: api.get(), api.post(), api.patch(), api.del()
 * đều trả về Promise với JSON đã parse sẵn.
 */

// Khi dev: Vite proxy /api → localhost:3001 (xem vite.config.ts)
// Khi production: nginx proxy cùng domain
const BASE_URL = "";

// ── Token refresh state (module-level, singleton) ──
let refreshPromise: Promise<boolean> | null = null;

/**
 * Gọi /api/auth/refresh để lấy access token mới.
 * Trả về true nếu refresh thành công, false nếu thất bại.
 * Các request song song cùng chia sẻ 1 promise để tránh refresh trùng lặp.
 */
const refreshAccessToken = async (): Promise<boolean> => {
  try {
    const res = await fetch(`${BASE_URL}/api/auth/refresh`, {
      method: "POST",
      credentials: "include",
    });
    return res.ok;
  } catch {
    return false;
  }
};

/**
 * Lấy (hoặc tạo) promise refresh — đảm bảo chỉ có 1 refresh chạy tại 1 thời điểm.
 */
const getRefreshPromise = (): Promise<boolean> => {
  if (!refreshPromise) {
    refreshPromise = refreshAccessToken().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
};

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

interface RequestOptions {
  headers?: Record<string, string>;
  /** Không tự động refresh khi gặp 401 (dùng cho chính endpoint refresh) */
  skipAuthRefresh?: boolean;
}

/**
 * Hàm request gốc — mọi GET/POST/PATCH/DELETE đều đi qua đây.
 * Tự động parse JSON, xử lý lỗi HTTP, và retry 1 lần nếu gặp 401.
 */
const request = async <T = unknown>(
  url: string,
  options: RequestInit & RequestOptions = {},
): Promise<T> => {
  const { headers: extraHeaders, skipAuthRefresh, ...fetchOptions } = options;

  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...(extraHeaders ?? {}),
  };

  const doFetch = (): Promise<Response> =>
    fetch(`${BASE_URL}${url}`, {
      ...fetchOptions,
      headers,
      credentials: "include",
    });

  let res = await doFetch();

  // Auto-refresh: nếu 401 và chưa skip → refresh rồi retry đúng 1 lần
  if (res.status === 401 && !skipAuthRefresh) {
    const refreshed = await getRefreshPromise();
    if (refreshed) {
      res = await doFetch();
    }
  }

  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      message = body.message ?? body.error ?? message;
    } catch {
      // Không parse được JSON → giữ message mặc định
    }
    throw new ApiError(res.status, message);
  }

  // 204 No Content → trả về undefined
  if (res.status === 204) {
    return undefined as T;
  }

  return res.json() as Promise<T>;
};

// ── Public API ──

export const api = {
  get: <T = unknown>(url: string, opts?: RequestOptions): Promise<T> =>
    request<T>(url, { method: "GET", ...opts }),

  post: <T = unknown>(
    url: string,
    body?: unknown,
    opts?: RequestOptions,
  ): Promise<T> =>
    request<T>(url, {
      method: "POST",
      body: body !== undefined ? JSON.stringify(body) : undefined,
      ...opts,
    }),

  patch: <T = unknown>(
    url: string,
    body?: unknown,
    opts?: RequestOptions,
  ): Promise<T> =>
    request<T>(url, {
      method: "PATCH",
      body: body !== undefined ? JSON.stringify(body) : undefined,
      ...opts,
    }),

  del: <T = unknown>(url: string, opts?: RequestOptions): Promise<T> =>
    request<T>(url, { method: "DELETE", ...opts }),
};

export default api;
