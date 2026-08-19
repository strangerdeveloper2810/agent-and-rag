/**
 * i18next resources — tách riêng khỏi index.ts vì file đó gọi getInitialLocale()
 * (dùng localStorage/window.navigator, chỉ tồn tại trên browser) ngay khi module
 * được import. Landing SSR (chạy trên Node lúc build) cần resources/defaultNS mà
 * KHÔNG được kéo theo side-effect đó, nên import module này thay vì "./index".
 */

import viCommon from "./locales/vi/common.json";
import viChat from "./locales/vi/chat.json";
import viAuth from "./locales/vi/auth.json";
import viDocuments from "./locales/vi/documents.json";
import viLayout from "./locales/vi/layout.json";
import viErrors from "./locales/vi/errors.json";
import viSettings from "./locales/vi/settings.json";
import viLanding from "./locales/vi/landing.json";

import enCommon from "./locales/en/common.json";
import enChat from "./locales/en/chat.json";
import enAuth from "./locales/en/auth.json";
import enDocuments from "./locales/en/documents.json";
import enLayout from "./locales/en/layout.json";
import enErrors from "./locales/en/errors.json";
import enSettings from "./locales/en/settings.json";
import enLanding from "./locales/en/landing.json";

export const defaultNS = "common";

export const resources = {
  vi: {
    common: viCommon,
    chat: viChat,
    auth: viAuth,
    documents: viDocuments,
    layout: viLayout,
    errors: viErrors,
    settings: viSettings,
    landing: viLanding,
  },
  en: {
    common: enCommon,
    chat: enChat,
    auth: enAuth,
    documents: enDocuments,
    layout: enLayout,
    errors: enErrors,
    settings: enSettings,
    landing: enLanding,
  },
} as const;
