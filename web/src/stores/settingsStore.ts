/**
 * Settings Store
 * User preferences and privacy settings with localStorage persistence
 */

import { create } from 'zustand';

interface SettingsState {
  /** Opt-out of telemetry collection */
  telemetryOptOut: boolean;
}

interface SettingsActions {
  setTelemetryOptOut: (optOut: boolean) => void;
  initializeSettings: () => void;
}

export type SettingsStore = SettingsState & SettingsActions;

/**
 * Local storage key for settings
 */
const SETTINGS_STORAGE_KEY = 'subcults-settings';

/**
 * Default settings
 */
const DEFAULT_SETTINGS: SettingsState = {
  telemetryOptOut: false,
};

/**
 * Load settings from localStorage
 */
function loadSettings(): SettingsState {
  try {
    const stored = localStorage.getItem(SETTINGS_STORAGE_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      return {
        telemetryOptOut: parsed.telemetryOptOut ?? DEFAULT_SETTINGS.telemetryOptOut,
      };
    }
  } catch (error) {
    console.warn('[settingsStore] Failed to load settings from localStorage:', error);
  }
  return DEFAULT_SETTINGS;
}

/**
 * Save settings to localStorage
 */
function saveSettings(settings: SettingsState): void {
  try {
    localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(settings));
  } catch (error) {
    console.warn('[settingsStore] Failed to save settings to localStorage:', error);
  }
}

/**
 * Settings store with localStorage persistence
 */
export const useSettingsStore = create<SettingsStore>((set, get) => ({
  ...DEFAULT_SETTINGS,

  setTelemetryOptOut: (optOut: boolean) => {
    set({ telemetryOptOut: optOut });
    saveSettings(get());
  },

  initializeSettings: () => {
    const loaded = loadSettings();
    set(loaded);
  },
}));

/**
 * Hook for telemetry opt-out flag only (optimized for re-renders)
 */
export function useTelemetryOptOut(): boolean {
  return useSettingsStore((state) => state.telemetryOptOut);
}

/**
 * Hook for settings actions only (stable reference)
 */
export function useSettingsActions() {
  const setTelemetryOptOut = useSettingsStore((state) => state.setTelemetryOptOut);
  const initializeSettings = useSettingsStore((state) => state.initializeSettings);

  return {
    setTelemetryOptOut,
    initializeSettings,
  };
}
