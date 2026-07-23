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

export default function App() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        {/* Home = hội thoại mới; /messages/:id = mở đúng phiên (reload vẫn giữ) */}
        <Route path="/" element={<ChatPage />} />
        <Route path="/messages/:id" element={<ChatPage />} />
        <Route path="/documents" element={<DocumentsView />} />
      </Route>
    </Routes>
  );
}
