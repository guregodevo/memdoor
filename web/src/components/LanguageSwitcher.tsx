import { useTranslation } from 'react-i18next';
import { supportedLanguages, type LanguageCode } from '../i18n';

interface LanguageSwitcherProps {
  variant?: 'dark' | 'light';
}

export default function LanguageSwitcher({ variant = 'dark' }: LanguageSwitcherProps) {
  const { i18n } = useTranslation();

  const changeLanguage = (lang: LanguageCode) => {
    i18n.changeLanguage(lang);
    const dir = supportedLanguages[lang].dir;
    document.documentElement.dir = dir;
    document.documentElement.lang = lang;
  };

  const className = variant === 'light'
    ? 'bg-white/20 text-white text-xs rounded px-2 py-1 border border-white/30 focus:outline-none backdrop-blur-sm cursor-pointer'
    : 'bg-gray-700 text-gray-300 text-xs rounded px-1 py-0.5 border border-gray-600 focus:outline-none';

  return (
    <select
      value={i18n.language}
      onChange={(e) => changeLanguage(e.target.value as LanguageCode)}
      className={className}
    >
      {Object.entries(supportedLanguages).map(([code, { name }]) => (
        <option key={code} value={code}>
          {name}
        </option>
      ))}
    </select>
  );
}
