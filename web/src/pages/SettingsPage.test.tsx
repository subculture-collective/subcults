/**
 * SettingsPage Component Tests
 * Tests settings page functionality and user interactions
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { SettingsPage } from './SettingsPage';
import { useThemeStore } from '../stores/themeStore';

describe('SettingsPage', () => {
  beforeEach(() => {
    localStorage.clear();
    useThemeStore.setState({ theme: 'light' });
    document.documentElement.classList.remove('dark');
  });

  const renderSettingsPage = () => {
    return render(
      <BrowserRouter>
        <SettingsPage />
      </BrowserRouter>
    );
  };

  describe('Rendering', () => {
    it('should render settings heading', () => {
      renderSettingsPage();
      expect(screen.getByRole('heading', { name: /^Settings$/i })).toBeInTheDocument();
    });

    it('should render appearance section', () => {
      renderSettingsPage();
      expect(screen.getByRole('heading', { name: /Appearance/i })).toBeInTheDocument();
    });

    it('should render privacy section', () => {
      renderSettingsPage();
      expect(screen.getByRole('heading', { name: /Privacy/i })).toBeInTheDocument();
    });

    it('should render notifications section', () => {
      renderSettingsPage();
      expect(screen.getByRole('heading', { name: /Notifications/i })).toBeInTheDocument();
    });

    it('should render theme preview section', () => {
      renderSettingsPage();
      expect(screen.getByRole('heading', { name: /Theme Preview/i })).toBeInTheDocument();
    });
  });

  describe('Appearance Settings', () => {
    it('should display theme label', () => {
      renderSettingsPage();
      expect(screen.getByText('Theme')).toBeInTheDocument();
    });

    it('should display theme description', () => {
      renderSettingsPage();
      expect(screen.getByText('Choose your preferred color scheme')).toBeInTheDocument();
    });

    it('should display current theme', () => {
      renderSettingsPage();
      expect(screen.getAllByText(/Current theme:/i)[0]).toBeInTheDocument();
      expect(screen.getByText('light')).toBeInTheDocument();
    });

    it('should render dark mode toggle component', () => {
      renderSettingsPage();
      // DarkModeToggle should be present
      const toggle = screen.getByLabelText(/dark mode/i);
      expect(toggle).toBeInTheDocument();
    });
  });

  describe('Privacy Settings', () => {
    it('should display privacy placeholder text', () => {
      renderSettingsPage();
      expect(
        screen.getByText(/Privacy settings and location consent preferences will be displayed here/i)
      ).toBeInTheDocument();
    });
  });

  describe('Theme Preview', () => {
    it('should render primary button', () => {
      renderSettingsPage();
      expect(screen.getByRole('button', { name: /Primary Button/i })).toBeInTheDocument();
    });

    it('should render accent button', () => {
      renderSettingsPage();
      expect(screen.getByRole('button', { name: /Accent Button/i })).toBeInTheDocument();
    });

    it('should render secondary button', () => {
      renderSettingsPage();
      expect(screen.getByRole('button', { name: /Secondary Button/i })).toBeInTheDocument();
    });

    it('should display preview description', () => {
      renderSettingsPage();
      expect(
        screen.getByText(/Preview how different UI elements look in the current theme/i)
      ).toBeInTheDocument();
    });

    it('should display primary text card', () => {
      renderSettingsPage();
      expect(screen.getByText('Primary Text')).toBeInTheDocument();
      expect(screen.getByText('Secondary text')).toBeInTheDocument();
      expect(screen.getByText('Muted text')).toBeInTheDocument();
    });

    it('should display underground card', () => {
      renderSettingsPage();
      expect(screen.getByText('Underground Card')).toBeInTheDocument();
      expect(screen.getByText('For dark aesthetic elements')).toBeInTheDocument();
    });

    it('should display brand card', () => {
      renderSettingsPage();
      expect(screen.getByText('Brand Card')).toBeInTheDocument();
      expect(screen.getByText('Primary brand colors')).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('should have proper heading hierarchy', () => {
      renderSettingsPage();

      const mainHeading = screen.getByRole('heading', { name: /^Settings$/i });
      expect(mainHeading.tagName).toBe('H1');

      const sectionHeadings = screen.getAllByRole('heading', { level: 2 });
      expect(sectionHeadings.length).toBeGreaterThan(0);
    });

    it('should have accessible sections with headings', () => {
      renderSettingsPage();

      const headings = screen.getAllByRole('heading');
      expect(headings.length).toBeGreaterThan(3); // At least Settings + 3 sections
    });
  });

  describe('Layout and Styling', () => {
    it('should apply theme-aware classes', () => {
      const { container } = renderSettingsPage();

      const mainContainer = container.querySelector('.min-h-screen');
      expect(mainContainer).toBeInTheDocument();
      expect(mainContainer).toHaveClass('bg-background', 'text-foreground');
    });

    it('should have proper spacing between sections', () => {
      const { container } = renderSettingsPage();

      const sectionsContainer = container.querySelector('.space-y-6');
      expect(sectionsContainer).toBeInTheDocument();
    });

    it('should render sections with borders and rounded corners', () => {
      const { container } = renderSettingsPage();

      const sections = container.querySelectorAll('section');
      sections.forEach((section) => {
        expect(section).toHaveClass('rounded-lg');
      });
    });
  });

  describe('Notifications Settings', () => {
    it('should render notification settings component', () => {
      renderSettingsPage();
      
      // NotificationSettings component should be rendered
      // It has its own heading
      expect(screen.getByRole('heading', { name: /Notifications/i })).toBeInTheDocument();
    });
  });

  describe('Theme State', () => {
    it('should display dark theme when store has dark theme', () => {
      useThemeStore.setState({ theme: 'dark' });
      
      renderSettingsPage();
      expect(screen.getByText('dark')).toBeInTheDocument();
    });
  });

  describe('User Experience', () => {
    it('should provide context about theme persistence', () => {
      renderSettingsPage();
      expect(
        screen.getByText(
          /Your theme preference is automatically saved and will be applied across all pages/i
        )
      ).toBeInTheDocument();
    });

    it('should clearly label theme preview section', () => {
      renderSettingsPage();
      expect(screen.getByText(/Preview how different UI elements look/i)).toBeInTheDocument();
    });
  });
});
