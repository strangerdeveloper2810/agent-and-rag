export type Locale = "vi" | "en";

export const LANGUAGE_STORAGE_KEY = "jarvis-lang";

const SUPPORTED_LOCALES: readonly Locale[] = ["vi", "en"];

const isSupportedLocale = (value: string | null): value is Locale =>
  value !== null && (SUPPORTED_LOCALES as readonly string[]).includes(value);

/**
 * Xác định locale ban đầu: ưu tiên giá trị đã lưu ở localStorage, sau đó
 * ngôn ngữ trình duyệt, mặc định "vi" (vì nội dung gốc của app là tiếng Việt).
 */
export const getInitialLocale = (): Locale => {
  const saved = localStorage.getItem(LANGUAGE_STORAGE_KEY);
  if (isSupportedLocale(saved)) return saved;

  const browserLang = window.navigator.language?.toLowerCase() ?? "";
  if (browserLang.startsWith("en")) return "en";

  return "vi";
};

export const persistLocale = (locale: Locale): void => {
  localStorage.setItem(LANGUAGE_STORAGE_KEY, locale);
};
