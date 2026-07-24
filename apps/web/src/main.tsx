import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App.tsx";
import { ToastProvider } from "@/shared/components/Toast";
import { initTheme } from "@/shared/components/ThemeToggle";
import "./index.css";

// Apply theme before React hydrates -- prevents dark mode flash on reload
initTheme();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <ToastProvider>
        <App />
      </ToastProvider>
    </BrowserRouter>
  </StrictMode>,
);
