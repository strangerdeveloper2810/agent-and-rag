export type RequestOptions = {
  headers?: Record<string, string>;
  signal?: AbortSignal;
  /** Timeout in milliseconds. Default 30_000 (30s). */
  timeout?: number;
  /** Number of retries on network error. Default 1 (total 2 attempts). */
  retries?: number;
};

export type RequestInterceptor = (
  url: string,
  init: RequestInit,
) => RequestInit | Promise<RequestInit>;

export type ResponseInterceptor = (
  response: Response,
) => Response | Promise<Response>;

export class HttpError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "HttpError";
    this.status = status;
  }
}

type Body = unknown;

export class HttpClient {
  private readonly baseURL: string;
  private requestInterceptors: RequestInterceptor[] = [];
  private responseInterceptors: ResponseInterceptor[] = [];

  constructor(baseURL = "") {
    this.baseURL = baseURL;
  }

  /** Add a request interceptor. Called before every fetch. */
  useRequest(fn: RequestInterceptor): () => void {
    this.requestInterceptors.push(fn);
    return () => {
      this.requestInterceptors = this.requestInterceptors.filter(
        (i) => i !== fn,
      );
    };
  }

  /** Add a response interceptor. Called after fetch, before JSON parsing. */
  useResponse(fn: ResponseInterceptor): () => void {
    this.responseInterceptors.push(fn);
    return () => {
      this.responseInterceptors = this.responseInterceptors.filter(
        (i) => i !== fn,
      );
    };
  }

  private buildInit(
    method: string,
    body?: Body,
    opts?: RequestOptions,
  ): RequestInit {
    const headers: Record<string, string> = { ...opts?.headers };
    let payload: BodyInit | undefined;

    if (typeof FormData !== "undefined" && body instanceof FormData) {
      payload = body;
    } else if (body !== undefined) {
      headers["content-type"] = headers["content-type"] ?? "application/json";
      payload = JSON.stringify(body);
    }

    return { method, headers, body: payload, signal: opts?.signal };
  }

  /** Core fetch with timeout and retry. */
  private async request(
    method: string,
    url: string,
    body?: Body,
    opts?: RequestOptions,
  ): Promise<Response> {
    const timeout = opts?.timeout ?? 30_000;
    const maxRetries = opts?.retries ?? 1;
    let lastError: Error | null = null;

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        let init = this.buildInit(method, body, opts);

        // Run request interceptors
        for (const interceptor of this.requestInterceptors) {
          init = await interceptor(url, init);
        }

        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), timeout);

        // Merge external signal with timeout signal
        const combinedSignal = opts?.signal
          ? anySignal([opts.signal, controller.signal])
          : controller.signal;

        try {
          let res = await fetch(this.baseURL + url, {
            ...init,
            signal: combinedSignal,
          });

          // Run response interceptors
          for (const interceptor of this.responseInterceptors) {
            res = await interceptor(res);
          }

          if (!res.ok) {
            const data = await res
              .json()
              .catch(() => ({}) as { error?: string });
            throw new HttpError(
              res.status,
              data?.error ?? `Request failed with status ${res.status}`,
            );
          }

          return res;
        } finally {
          clearTimeout(timeoutId);
        }
      } catch (err) {
        lastError = err as Error;

        // Don't retry if aborted (timeout or user cancel)
        if (lastError.name === "AbortError") throw lastError;
        // Don't retry client errors (4xx except 429)
        if (
          lastError instanceof HttpError &&
          lastError.status >= 400 &&
          lastError.status < 500 &&
          lastError.status !== 429
        ) {
          throw lastError;
        }
        // Last attempt -> throw
        if (attempt === maxRetries) throw lastError;

        // Exponential backoff: 200ms, 400ms, 800ms...
        await sleep(Math.min(200 * Math.pow(2, attempt), 3000));
      }
    }

    throw lastError ?? new Error("Request failed");
  }

  /** Return raw Response -- for streaming (SSE) or manual body handling. */
  async raw(
    method: string,
    url: string,
    body?: Body,
    opts?: RequestOptions,
  ): Promise<Response> {
    return this.request(method, url, body, opts);
  }

  private async json<T>(
    method: string,
    url: string,
    body?: Body,
    opts?: RequestOptions,
  ): Promise<T> {
    const res = await this.request(method, url, body, opts);
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
  /** POST returning raw Response for SSE streaming. */
  stream(url: string, body?: Body, opts?: RequestOptions) {
    return this.raw("POST", url, body, opts);
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function anySignal(signals: AbortSignal[]): AbortSignal {
  const controller = new AbortController();
  for (const signal of signals) {
    if (signal.aborted) {
      controller.abort(signal.reason);
      return controller.signal;
    }
    signal.addEventListener("abort", () => controller.abort(signal.reason), {
      once: true,
    });
  }
  return controller.signal;
}

export function createHttpClient(baseURL = ""): HttpClient {
  return new HttpClient(baseURL);
}
