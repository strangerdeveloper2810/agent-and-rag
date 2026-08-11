import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App.tsx";
import { ToastProvider } from "@/design-system/molecules/Toast";
import { initTheme } from "@/design-system/atoms/ThemeToggle";
import { useAuthStore } from "@/stores/auth.store";
import "./index.css";

initTheme();

// Khởi tạo auth — kiểm tra session hiện có (nếu có cookie còn hạn)
useAuthStore.getState().init();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <ToastProvider>
        <App />
      </ToastProvider>
    </BrowserRouter>
  </StrictMode>,
);
