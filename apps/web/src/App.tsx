import { lazy } from "react";
import { Routes, Route } from "react-router-dom";
import AppLayout from "@/design-system/templates/AppLayout";
import AuthGuard from "@/components/guards/AuthGuard";
import GuestGuard from "@/components/guards/GuestGuard";
import { ErrorBoundary } from "@/components/ErrorBoundary";

// Lazy-load trang theo route → code-splitting: bundle vào đầu nhẹ hơn.
// Suspense đặt trong AppLayout (quanh Outlet) nên sidebar vẫn hiển thị khi chunk đang tải.
const ChatPage = lazy(() => import("@/modules/chat/components/ChatPage"));
const DocumentsView = lazy(
  () => import("@/modules/documents/components/DocumentsView"),
);
const LoginPage = lazy(() => import("@/pages/auth/LoginPage"));
const RegisterPage = lazy(() => import("@/pages/auth/RegisterPage"));
const VerifyEmailPage = lazy(() => import("@/pages/auth/VerifyEmailPage"));
const ForgotPasswordPage = lazy(
  () => import("@/pages/auth/ForgotPasswordPage"),
);
// Preview-only route để visual review thiết kế landing page trước khi chốt
// implementation đầy đủ (xem docs/plans/2026-08-19-landing-page-ssr-design.md).
const LandingHome = lazy(() => import("@/modules/landing/LandingHome"));

/**
 * Main application component configuring React Router routes with lazy loading & code splitting.
 */
export const App: React.FC = () => {
  return (
    <ErrorBoundary>
      <Routes>
        {/* Preview-only, public, không guard — chỉ để xem trước UI landing page */}
        <Route path="/landing-preview" element={<LandingHome />} />

        {/* Auth pages — chặn user đã đăng nhập, không có sidebar */}
        <Route
          path="/login"
          element={
            <GuestGuard>
              <LoginPage />
            </GuestGuard>
          }
        />
        <Route
          path="/register"
          element={
            <GuestGuard>
              <RegisterPage />
            </GuestGuard>
          }
        />
        <Route
          path="/verify-email"
          element={
            <GuestGuard>
              <VerifyEmailPage />
            </GuestGuard>
          }
        />
        <Route
          path="/forgot-password"
          element={
            <GuestGuard>
              <ForgotPasswordPage />
            </GuestGuard>
          }
        />

        {/* Protected routes — wrapped với AuthGuard + AppLayout */}
        <Route
          element={
            <AuthGuard>
              <AppLayout />
            </AuthGuard>
          }
        >
          <Route path="/" element={<ChatPage />} />
          <Route path="/messages/:id" element={<ChatPage />} />
          <Route path="/documents" element={<DocumentsView />} />
        </Route>
      </Routes>
    </ErrorBoundary>
  );
};

export default App;
