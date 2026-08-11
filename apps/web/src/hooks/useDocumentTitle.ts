/**
 * useDocumentTitle — cập nhật document.title động theo route.
 *
 * Cách dùng:
 *   useDocumentTitle("Đăng nhập");
 *   useDocumentTitle("Chat");           // → "Chat | JARVIS"
 *   useDocumentTitle(undefined);        // reset về default
 */

import { useEffect } from "react";

const APP_NAME = "JARVIS";
const DEFAULT_TITLE = APP_NAME;

/**
 * Set document.title với format "{title} | {APP_NAME}".
 * Nếu `title` là undefined/empty → reset về "JARVIS".
 * Tự động khôi phục title cũ khi component unmount.
 */
export const useDocumentTitle = (title?: string): void => {
  useEffect(() => {
    const prev = document.title;

    document.title = title ? `${title} | ${APP_NAME}` : DEFAULT_TITLE;

    return () => {
      document.title = prev;
    };
  }, [title]);
};

export default useDocumentTitle;
