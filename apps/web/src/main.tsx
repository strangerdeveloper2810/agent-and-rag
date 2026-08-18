import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { I18nextProvider } from "react-i18next";
import { QueryClientProvider } from "@tanstack/react-query";
import App from "./App.tsx";
import { ToastProvider } from "@/design-system/molecules/Toast";
import { initTheme } from "@/design-system/atoms/ThemeToggle";
import { queryClient } from "@/lib/query-client";
import i18n from "@/i18n";
import "./index.css";

initTheme();

// Session KHÔNG còn được nạp ở đây nữa. Trước đây file này gọi
// useAuthStore.getState().init() ở module scope, cộng thêm mỗi guard gọi
// init() trong useEffect → reload trang là 3 lần GET /api/auth/me. Giờ guard
// dùng useSession() (TanStack Query) nên mọi chỗ chia sẻ đúng một request.
//
// queryClient phải là ĐÚNG instance mà auth.store ghi vào (khi login/logout),
// nên dùng singleton từ @/lib/query-client thay vì tạo mới ở đây.

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <BrowserRouter>
          <ToastProvider>
            <App />
          </ToastProvider>
        </BrowserRouter>
      </I18nextProvider>
    </QueryClientProvider>
  </StrictMode>,
);
