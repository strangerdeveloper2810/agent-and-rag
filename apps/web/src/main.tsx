import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { I18nextProvider } from "react-i18next";
import App from "./App.tsx";
import { ToastProvider } from "@/design-system/molecules/Toast";
import { initTheme } from "@/design-system/atoms/ThemeToggle";
import { useAuthStore } from "@/stores/auth.store";
import i18n from "@/i18n";
import "./index.css";

initTheme();

// Khởi tạo auth — kiểm tra session hiện có (nếu có cookie còn hạn)
useAuthStore.getState().init();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <I18nextProvider i18n={i18n}>
      <BrowserRouter>
        <ToastProvider>
          <App />
        </ToastProvider>
      </BrowserRouter>
    </I18nextProvider>
  </StrictMode>,
);
