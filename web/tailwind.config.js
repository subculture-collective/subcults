/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class', // Enable class-based dark mode
  theme: {
    extend: {
      screens: {
        'xs': '375px',   // iPhone SE and similar
      },
      colors: {
        // Brand colors for underground music aesthetic
        brand: {
          primary: {
            DEFAULT: '#646cff', // Electric blue
            light: '#747bff',
            dark: '#535bf2',
          },
          accent: {
            DEFAULT: '#61dafb', // React cyan
            light: '#80e5ff',
            dark: '#42c5e8',
          },
          underground: {
            DEFAULT: '#1a1a1a', // Deep dark
            light: '#242424',
            lighter: '#2d2d2d',
          },
        },
        // Semantic colors with dark mode variants
        background: {
          DEFAULT: 'var(--color-background)',
          secondary: 'var(--color-background-secondary)',
        },
        foreground: {
          DEFAULT: 'var(--color-foreground)',
          secondary: 'var(--color-foreground-secondary)',
          muted: 'var(--color-foreground-muted)',
        },
        border: {
          DEFAULT: 'var(--color-border)',
          hover: 'var(--color-border-hover)',
        },
      },
      spacing: {
        '11': '2.75rem',  // 44px - minimum touch target
        '18': '4.5rem',
        '88': '22rem',
        '128': '32rem',
      },
      minHeight: {
        'touch': '44px',  // Minimum touch target height
      },
      minWidth: {
        'touch': '44px',  // Minimum touch target width
      },
      fontSize: {
        '2xs': ['0.625rem', { lineHeight: '0.875rem' }],
      },
      fontFamily: {
        sans: ['system-ui', 'Avenir', 'Helvetica', 'Arial', 'sans-serif'],
      },
      borderRadius: {
        '4xl': '2rem',
      },
      transitionDuration: {
        '250': '250ms',
      },
      animation: {
        'fade-in': 'fadeIn 0.2s ease-in',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-in': 'slideIn 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { transform: 'translateY(10px)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' },
        },
        slideIn: {
          '0%': { transform: 'translateX(100%)', opacity: '0' },
          '100%': { transform: 'translateX(0)', opacity: '1' },
        },
        slideInRight: {
          '0%': { transform: 'translateX(100%)', opacity: '0' },
          '100%': { transform: 'translateX(0)', opacity: '1' },
        },
      },
    },
  },
  plugins: [],
}
