/**
 * SettingsPage Component
 * User settings and preferences
 */

import React from 'react';
import { DarkModeToggle } from '../components/DarkModeToggle';
import { useTheme } from '../stores/themeStore';

export const SettingsPage: React.FC = () => {
  const theme = useTheme();
  
  return (
    <div className="min-h-screen bg-background text-foreground p-8">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-4xl font-bold mb-8 text-foreground">Settings</h1>
        
        <div className="space-y-6">
          {/* Appearance Section */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">Appearance</h2>
            
            <div className="flex items-center justify-between py-4 border-b border-border">
              <div>
                <h3 className="text-lg font-medium text-foreground mb-1">Theme</h3>
                <p className="text-sm text-foreground-secondary">
                  Choose your preferred color scheme
                </p>
              </div>
              <DarkModeToggle showLabel={true} />
            </div>
            
            <div className="py-4">
              <p className="text-sm text-foreground-muted">
                Current theme: <span className="font-semibold text-foreground">{theme}</span>
              </p>
              <p className="text-sm text-foreground-muted mt-2">
                Your theme preference is automatically saved and will be applied across all pages.
              </p>
            </div>
          </section>
          
          {/* Privacy Section */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">Privacy</h2>
            <p className="text-foreground-secondary">
              Privacy settings and location consent preferences will be displayed here.
            </p>
          </section>
          
          {/* Notifications Section */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">Notifications</h2>
            <p className="text-foreground-secondary">
              Notification preferences and alert settings will be displayed here.
            </p>
          </section>
          
          {/* Demo Section - Show theme colors */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">Theme Preview</h2>
            <p className="text-foreground-secondary mb-4">
              Preview how different UI elements look in the current theme:
            </p>
            
            <div className="space-y-4">
              <div className="flex gap-4 flex-wrap">
                <button className="bg-brand-primary hover:bg-brand-primary-dark text-white px-4 py-2 rounded-lg transition-colors">
                  Primary Button
                </button>
                <button className="bg-brand-accent hover:bg-brand-accent-dark text-white px-4 py-2 rounded-lg transition-colors">
                  Accent Button
                </button>
                <button className="bg-background-secondary hover:bg-underground-lighter border border-border text-foreground px-4 py-2 rounded-lg transition-colors">
                  Secondary Button
                </button>
              </div>
              
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
                <div className="bg-background border border-border p-4 rounded-lg">
                  <p className="text-foreground font-semibold">Primary Text</p>
                  <p className="text-foreground-secondary text-sm mt-2">Secondary text</p>
                  <p className="text-foreground-muted text-sm mt-1">Muted text</p>
                </div>
                
                <div className="bg-brand-underground border border-border p-4 rounded-lg">
                  <p className="text-white font-semibold">Underground Card</p>
                  <p className="text-gray-300 text-sm mt-2">For dark aesthetic elements</p>
                </div>
                
                <div className="bg-brand-primary border border-brand-primary-dark p-4 rounded-lg">
                  <p className="text-white font-semibold">Brand Card</p>
                  <p className="text-blue-100 text-sm mt-2">Primary brand colors</p>
                </div>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
};
