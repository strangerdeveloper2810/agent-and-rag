import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import { getInitialLocale } from "./locale";
import { defaultNS, resources } from "./resources";

export { defaultNS, resources };

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
