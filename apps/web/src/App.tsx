import { Routes, Route } from "react-router-dom";
import AppLayout from "./components/AppLayout";
import ChatPage from "./components/ChatPage";
import DocumentsView from "./components/DocumentsView";

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
