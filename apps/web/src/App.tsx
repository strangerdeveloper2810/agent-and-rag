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

/**
 * Main application component configuring React Router routes with lazy loading & code splitting.
 *
 * Landing page ("/", "/pricing", "/features") KHÔNG còn nằm trong bundle này —
 * đã tách thành bundle SSG riêng (xem src/modules/landing/entry-landing-*.tsx +
 * scripts/prerender.mjs + docs/plans/2026-08-19-landing-page-ssr-design.md).
 * App này chỉ còn phục vụ /login, /register, /verify-email, /forgot-password,
 * và /app/* (chat CSR sau khi đăng nhập).
 */
export const App: React.FC = () => {
  return (
    <ErrorBoundary>
      <Routes>
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

        {/* Protected routes — wrapped với AuthGuard + AppLayout, dưới /app */}
        <Route
          element={
            <AuthGuard>
              <AppLayout />
            </AuthGuard>
          }
        >
          <Route path="/app" element={<ChatPage />} />
          <Route path="/app/messages/:id" element={<ChatPage />} />
          <Route path="/app/documents" element={<DocumentsView />} />
        </Route>
      </Routes>
    </ErrorBoundary>
  );
};

export default App;
