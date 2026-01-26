/**
 * Language Store
 * User language preference with localStorage persistence
 */

import { create } from 'zustand';
import i18n from '../i18n';

export type Language = 'en' | 'es';

interface LanguageState {
  /** Current language */
  language: Language;
}

interface LanguageActions {
  /** Change the current language */
  setLanguage: (language: Language) => void;
  /** Initialize language from storage or browser */
  initializeLanguage: () => void;
}

export type LanguageStore = LanguageState & LanguageActions;

/**
 * Local storage key for language preference
 */
const LANGUAGE_STORAGE_KEY = 'subcults-language';

/**
 * Detect language using fallback chain:
 * 1. User setting (localStorage)
 * 2. Browser navigator.languages
 * 3. Fallback to 'en'
 */
function detectLanguage(): Language {
  // 1. Check localStorage for user preference
  const stored = localStorage.getItem(LANGUAGE_STORAGE_KEY);
  if (stored === 'en' || stored === 'es') {
    return stored;
  }

  // 2. Check browser languages
  if (typeof navigator !== 'undefined' && navigator.languages) {
    for (const lang of navigator.languages) {
      // Match exact language or language prefix
      const langCode = lang.toLowerCase().split('-')[0];
      if (langCode === 'es') {
        return 'es';
      }
      if (langCode === 'en') {
        return 'en';
      }
    }
  }

  // 3. Fallback to English
  return 'en';
}

/**
 * Language store with localStorage persistence
 * 
 * IMPORTANT: Call initializeLanguage() on app startup to detect and apply
 * the user's preferred language.
 * 
 * Example:
 * ```typescript
 * // In App.tsx or root component:
 * useEffect(() => {
 *   useLanguageStore.getState().initializeLanguage();
 * }, []);
 * ```
 */
export const useLanguageStore = create<LanguageStore>((set) => ({
  language: 'en',

  setLanguage: (language: Language) => {
    set({ language });
    // Persist to localStorage
    localStorage.setItem(LANGUAGE_STORAGE_KEY, language);
    // Update i18n
    i18n.changeLanguage(language);
  },

  initializeLanguage: () => {
    const detected = detectLanguage();
    set({ language: detected });
    // Update i18n but don't persist (only persist explicit user changes)
    i18n.changeLanguage(detected);
  },
}));

/**
 * Hook for current language only (optimized for re-renders)
 */
export function useLanguage(): Language {
  return useLanguageStore((state) => state.language);
}

/**
 * Hook for language actions only (stable reference)
 */
export function useLanguageActions() {
  const setLanguage = useLanguageStore((state) => state.setLanguage);
  const initializeLanguage = useLanguageStore((state) => state.initializeLanguage);

  return {
    setLanguage,
    initializeLanguage,
  };
}
