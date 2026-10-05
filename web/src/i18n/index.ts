import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { getApiUrl } from '../config';

import en from './locales/en.json';
import ar from './locales/ar.json';
import fr from './locales/fr.json';
import es from './locales/es.json';
import ru from './locales/ru.json';
import pt from './locales/pt.json';
import hi from './locales/hi.json';
import zh from './locales/zh.json';

export const supportedLanguages = {
  en: { name: 'English', dir: 'ltr' },
  ar: { name: 'العربية', dir: 'rtl' },
  fr: { name: 'Français', dir: 'ltr' },
  es: { name: 'Español', dir: 'ltr' },
  ru: { name: 'Русский', dir: 'ltr' },
  pt: { name: 'Português', dir: 'ltr' },
  hi: { name: 'हिन्दी', dir: 'ltr' },
  zh: { name: '中文', dir: 'ltr' },
} as const;

export type LanguageCode = keyof typeof supportedLanguages;

// Fetch workspace default language and apply it if user has no override
export async function applyWorkspaceLanguage() {
  try {
    const res = await fetch(getApiUrl('/api/workspace/settings/public'));
    if (!res.ok) return;
    const data = await res.json();
    const wsLang = data.settings?.language;
    if (!wsLang || !(wsLang in supportedLanguages)) return;

    // Only apply workspace language if user hasn't set a personal preference
    const userOverride = localStorage.getItem('i18nLng');
    if (!userOverride) {
      await i18n.changeLanguage(wsLang);
      document.documentElement.dir = supportedLanguages[wsLang as LanguageCode].dir;
      document.documentElement.lang = wsLang;
    }
  } catch {
    // Workspace settings not available, keep current language
  }
}

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: { translation: en },
      ar: { translation: ar },
      fr: { translation: fr },
      es: { translation: es },
      ru: { translation: ru },
      pt: { translation: pt },
      hi: { translation: hi },
      zh: { translation: zh },
    },
    fallbackLng: 'en',
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
    },
  });

export default i18n;
