import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App.tsx";
import { ToastProvider } from "@/shared/components/Toast";
import ScanlineOverlay from "@/shared/components/ScanlineOverlay";
import { initTheme } from "@/shared/components/ThemeToggle";
import "./index.css";

initTheme();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <ToastProvider>
        <ScanlineOverlay />
        <App />
      </ToastProvider>
    </BrowserRouter>
  </StrictMode>,
);
