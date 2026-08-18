/**
 * Helper cho test khi component đọc server state qua TanStack Query.
 *
 * Mỗi test PHẢI có QueryClient riêng: client dùng chung sẽ mang cache của test
 * trước sang test sau, làm test đậu/đổ tuỳ thứ tự chạy.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement, ReactNode } from "react";

/**
 * QueryClient cho môi trường test: không retry (test lỗi phải đổ ngay chứ
 * không chờ thử lại), staleTime vô hạn (chỉ fetch khi test chủ động cho phép),
 * và không tự dọn cache giữa các assertion.
 */
export const createTestQueryClient = (): QueryClient =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity, gcTime: Infinity },
      mutations: { retry: false },
    },
  });

/** Bọc UI trong QueryClientProvider — dùng khi test tự lo các provider khác. */
export const withQueryClient = (
  ui: ReactNode,
  queryClient: QueryClient = createTestQueryClient(),
): ReactElement => (
  <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
);
