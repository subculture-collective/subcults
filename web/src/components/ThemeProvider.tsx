/**
 * ThemeProvider Component
 * Initializes dark theme on mount
 */

import { useEffect } from 'react';
import { useThemeActions } from '../stores/themeStore';

interface ThemeProviderProps {
  children: React.ReactNode;
}

/**
 * ThemeProvider wraps the app and manages theme initialization
 */
export function ThemeProvider({ children }: ThemeProviderProps) {
  const { initializeTheme } = useThemeActions();

  useEffect(() => {
    // Initialize component permanently in dark mode
    initializeTheme();
  }, [initializeTheme]);

  return <>{children}</>;
}
