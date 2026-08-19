import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import { getInitialLocale } from "./locale";

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

void i18n.use(initReactI18next).init({
  lng: getInitialLocale(),
  fallbackLng: "vi",
  defaultNS,
  ns: Object.keys(resources.vi),
  resources,
  interpolation: {
    escapeValue: false,
  },
});

export default i18n;
