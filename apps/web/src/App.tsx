import { lazy } from "react";
import { Routes, Route } from "react-router-dom";
import AppLayout from "@/shared/components/AppLayout";

// Lazy-load trang theo route → code-splitting: bundle vào đầu nhẹ hơn, trang
// Documents (và các dep của nó) chỉ tải khi thực sự mở. Suspense đặt trong
// AppLayout (quanh Outlet) nên sidebar vẫn hiển thị khi chunk đang tải.
const ChatPage = lazy(() => import("@/modules/chat/components/ChatPage"));
const DocumentsView = lazy(
  () => import("@/modules/documents/components/DocumentsView"),
);

/**
 * Main application component configuring React Router routes with lazy loading & code splitting.
 */
export const App: React.FC = () => {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        {/* Home = new chat; /messages/:id = active session */}
        <Route path="/" element={<ChatPage />} />
        <Route path="/messages/:id" element={<ChatPage />} />
        <Route path="/documents" element={<DocumentsView />} />
      </Route>
    </Routes>
  );
};

export default App;
