/**
 * useDocumentTitle — cập nhật document.title động theo route.
 *
 * Cách dùng:
 *   useDocumentTitle("Đăng nhập");
 *   useDocumentTitle("Chat");           // → "Chat | JARVIS"
 *   useDocumentTitle(undefined);        // reset về default
 */

import { useEffect } from "react";

const APP_NAME = "J.A.R.V.I.S.";
const DEFAULT_TITLE = `${APP_NAME} — Trợ Lý AI Thông Minh & Đa Tác Nhân`;

/**
 * Set document.title với format "{title} — {APP_NAME}".
 * Nếu `title` là undefined/empty → reset về DEFAULT_TITLE.
 * Tự động cập nhật thẻ meta description nếu có.
 */
export const useDocumentTitle = (title?: string, description?: string): void => {
  useEffect(() => {
    const prev = document.title;
    document.title = title ? `${title} — ${APP_NAME}` : DEFAULT_TITLE;

    const metaDesc = document.querySelector('meta[name="description"]');
    const prevDesc = metaDesc ? metaDesc.getAttribute("content") : null;
    if (description && metaDesc) {
      metaDesc.setAttribute("content", description);
    }

    return () => {
      document.title = prev;
      if (prevDesc && metaDesc) {
        metaDesc.setAttribute("content", prevDesc);
      }
    };
  }, [title, description]);
};

export default useDocumentTitle;
