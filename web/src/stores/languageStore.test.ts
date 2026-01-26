/**
 * Language Store Tests
 * Validates language preference state management and detection
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useLanguageStore, useLanguage, useLanguageActions } from './languageStore';
import i18n from '../i18n';

describe('languageStore', () => {
  beforeEach(() => {
    // Clear localStorage
    localStorage.clear();
    // Reset store to initial state
    useLanguageStore.setState({ language: 'en' });
    // Reset i18n
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('language detection', () => {
    it('uses stored language from localStorage', () => {
      localStorage.setItem('subcults-language', 'es');
      const { result } = renderHook(() => useLanguageStore());

      act(() => {
        result.current.initializeLanguage();
      });

      expect(result.current.language).toBe('es');
    });

    it('detects Spanish from browser languages', () => {
      // Mock navigator.languages
      Object.defineProperty(navigator, 'languages', {
        writable: true,
        value: ['es-MX', 'es', 'en'],
      });

      const { result } = renderHook(() => useLanguageStore());

      act(() => {
        result.current.initializeLanguage();
      });

      expect(result.current.language).toBe('es');
    });

    it('detects English from browser languages', () => {
      Object.defineProperty(navigator, 'languages', {
        writable: true,
        value: ['en-US', 'en'],
      });

      const { result } = renderHook(() => useLanguageStore());

      act(() => {
        result.current.initializeLanguage();
      });

      expect(result.current.language).toBe('en');
    });

    it('falls back to English when no preference available', () => {
      Object.defineProperty(navigator, 'languages', {
        writable: true,
        value: ['fr-FR', 'de'],
      });

      const { result } = renderHook(() => useLanguageStore());

      act(() => {
        result.current.initializeLanguage();
      });

      expect(result.current.language).toBe('en');
    });

    it('prioritizes localStorage over browser languages', () => {
      localStorage.setItem('subcults-language', 'en');
      Object.defineProperty(navigator, 'languages', {
        writable: true,
        value: ['es-MX', 'es'],
      });

      const { result } = renderHook(() => useLanguageStore());

      act(() => {
        result.current.initializeLanguage();
      });

      expect(result.current.language).toBe('en');
    });
  });

  describe('setLanguage', () => {
    it('updates language state', () => {
      const { result } = renderHook(() => useLanguageStore());

      act(() => {
        result.current.setLanguage('es');
      });

      expect(result.current.language).toBe('es');
    });

    it('persists language to localStorage', () => {
      const { result } = renderHook(() => useLanguageStore());

      act(() => {
        result.current.setLanguage('es');
      });

      expect(localStorage.getItem('subcults-language')).toBe('es');
    });

    it('updates i18n language', () => {
      const changeLanguageSpy = vi.spyOn(i18n, 'changeLanguage');
      const { result } = renderHook(() => useLanguageStore());

      act(() => {
        result.current.setLanguage('es');
      });

      expect(changeLanguageSpy).toHaveBeenCalledWith('es');
    });
  });

  describe('useLanguage hook', () => {
    it('returns current language', () => {
      useLanguageStore.setState({ language: 'es' });
      const { result } = renderHook(() => useLanguage());

      expect(result.current).toBe('es');
    });
  });

  describe('useLanguageActions hook', () => {
    it('returns stable action references', () => {
      const { result, rerender } = renderHook(() => useLanguageActions());
      const firstRender = result.current;

      rerender();
      const secondRender = result.current;

      // Actions should maintain referential equality
      expect(firstRender.setLanguage).toBe(secondRender.setLanguage);
      expect(firstRender.initializeLanguage).toBe(secondRender.initializeLanguage);
    });
  });
});
