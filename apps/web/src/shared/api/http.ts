import { createHttpClient, HttpClient, HttpError } from "@app/http";
export type { RequestOptions, RequestInterceptor, ResponseInterceptor } from "@app/http";
export { HttpClient, HttpError };

// Singleton: all requests go through this instance, baseURL = "/api" (Vite proxy -> :3001)
export const http: HttpClient = createHttpClient("/api");
