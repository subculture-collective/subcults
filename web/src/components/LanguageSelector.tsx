/**
 * Language Selector Component
 * Allows users to switch between available languages
 */

import React from 'react';
import { useLanguage, useLanguageActions, type Language } from '../stores/languageStore';
import { useTranslation } from 'react-i18next';

export const LanguageSelector: React.FC = () => {
  const currentLanguage = useLanguage();
  const { setLanguage } = useLanguageActions();
  const { t } = useTranslation('common');

  const handleChange = (event: React.ChangeEvent<HTMLSelectElement>) => {
    setLanguage(event.target.value as Language);
  };

  return (
    <div className="language-selector">
      <label htmlFor="language-select" className="sr-only">
        {t('language.select')}
      </label>
      <select
        id="language-select"
        value={currentLanguage}
        onChange={handleChange}
        className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
        aria-label={t('language.select')}
      >
        <option value="en">{t('language.en')}</option>
        <option value="es">{t('language.es')}</option>
        <option value="fr">{t('language.fr')}</option>
        <option value="de">{t('language.de')}</option>
      </select>
    </div>
  );
};

export default LanguageSelector;
