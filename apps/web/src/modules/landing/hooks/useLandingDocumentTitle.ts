import { useEffect } from "react";

/**
 * useLandingDocumentTitle — set document.title trực tiếp, KHÁC với
 * @/hooks/useDocumentTitle (hook đó tự nối thêm "— J.A.R.V.I.S." vì các trang
 * app CSR truyền vào title ngắn không có brand, vd "Đăng nhập"). Landing page
 * đã tự quyết định toàn bộ chuỗi title (gồm cả brand) khớp CHÍNH XÁC với những
 * gì scripts/prerender.mjs chèn vào HTML tĩnh lúc build — nối thêm brand ở đây
 * sẽ bị lặp ("... — J.A.R.V.I.S. — J.A.R.V.I.S.") ngay sau khi hydrate.
 */
export function useLandingDocumentTitle(
  title: string,
  description?: string,
): void {
  useEffect(() => {
    document.title = title;
    if (description) {
      document
        .querySelector('meta[name="description"]')
        ?.setAttribute("content", description);
    }
  }, [title, description]);
}

export default useLandingDocumentTitle;
