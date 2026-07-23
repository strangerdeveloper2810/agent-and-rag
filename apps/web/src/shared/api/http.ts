// HTTP client dùng chung (singleton). Nhận url + payload (JSON/FormData) + header tùy chọn.
// - body là object → tự JSON.stringify + content-type application/json
// - body là FormData → để nguyên (browser tự set boundary)
// - lỗi (!res.ok) → ném HttpError mang theo status + message (lấy field `error` từ server)

export type RequestOptions = {
  headers?: Record<string, string>;
  signal?: AbortSignal;
};

export class HttpError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "HttpError";
    this.status = status;
  }
}

type Body = unknown;

class HttpClient {
  constructor(private readonly baseURL = "") {}

  private buildInit(
    method: string,
    body?: Body,
    opts?: RequestOptions,
  ): RequestInit {
    const headers: Record<string, string> = { ...opts?.headers };
    let payload: BodyInit | undefined;

    if (body instanceof FormData) {
      payload = body;
    } else if (body !== undefined) {
      headers["content-type"] = headers["content-type"] ?? "application/json";
      payload = JSON.stringify(body);
    }

    return { method, headers, body: payload, signal: opts?.signal };
  }

  // Trả Response thô — dùng cho stream (SSE) hoặc khi cần tự xử lý body
  async raw(
    method: string,
    url: string,
    body?: Body,
    opts?: RequestOptions,
  ): Promise<Response> {
    const res = await fetch(
      this.baseURL + url,
      this.buildInit(method, body, opts),
    );
    if (!res.ok) {
      const data = await res.json().catch(() => ({}) as { error?: string });
      throw new HttpError(res.status, data?.error ?? res.statusText);
    }
    return res;
  }

  private async json<T>(
    method: string,
    url: string,
    body?: Body,
    opts?: RequestOptions,
  ): Promise<T> {
    const res = await this.raw(method, url, body, opts);
    if (res.status === 204) return undefined as T;
    return res.json() as Promise<T>;
  }

  get<T>(url: string, opts?: RequestOptions) {
    return this.json<T>("GET", url, undefined, opts);
  }
  post<T>(url: string, body?: Body, opts?: RequestOptions) {
    return this.json<T>("POST", url, body, opts);
  }
  put<T>(url: string, body?: Body, opts?: RequestOptions) {
    return this.json<T>("PUT", url, body, opts);
  }
  delete<T>(url: string, opts?: RequestOptions) {
    return this.json<T>("DELETE", url, undefined, opts);
  }
  // POST trả Response thô để đọc stream (SSE)
  stream(url: string, body?: Body, opts?: RequestOptions) {
    return this.raw("POST", url, body, opts);
  }
}

// Singleton: mọi request đi qua đây, baseURL = "/api" (Vite proxy → :3001)
export const http = new HttpClient("/api");
