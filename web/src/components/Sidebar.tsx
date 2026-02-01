/**
 * Sidebar Component
 * Navigation sidebar with scene list and quick actions
 */

import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../stores/authStore';

// Read version from package.json at build time
const VERSION = import.meta.env.VITE_APP_VERSION || '0.1.0';

export interface SidebarProps {
  /**
   * Whether the sidebar is open (for mobile)
   */
  isOpen?: boolean;
  /**
   * Callback when sidebar should close (mobile)
   */
  onClose?: () => void;
  /**
   * Additional CSS classes
   */
  className?: string;
}

/**
 * Sidebar provides navigation and quick access to scenes
 */
export function Sidebar({ isOpen = true, onClose, className = '' }: SidebarProps) {
  const location = useLocation();
  const { isAuthenticated, isAdmin } = useAuth();

  const navItems = [
    { path: '/', label: 'Map', icon: '🗺️' },
    { path: '/scenes', label: 'Scenes', icon: '🎭' },
    { path: '/events', label: 'Events', icon: '📅' },
  ];

  const accountItems = [
    ...(isAuthenticated ? [{ path: '/account', label: 'Account', icon: '👤' }] : []),
    { path: '/settings', label: 'Settings', icon: '⚙️' },
    ...(isAdmin ? [{ path: '/admin', label: 'Admin', icon: '🔧' }] : []),
  ];

  const isActive = (path: string) => {
    if (path === '/') {
      return location.pathname === '/';
    }
    return location.pathname.startsWith(path);
  };

  const handleLinkClick = () => {
    onClose?.();
  };

  return (
    <>
      {/* Mobile Overlay */}
      {isOpen && onClose && (
        <div
          className="fixed inset-0 bg-black/50 z-40 lg:hidden"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
          fixed lg:static inset-y-0 left-0 z-50
          w-64 bg-background-secondary border-r border-border
          transform transition-transform duration-300 ease-in-out
          lg:transform-none
          ${isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
          ${className}
        `}
        aria-label="Sidebar navigation"
      >
        <div className="flex flex-col h-full overflow-y-auto">
          {/* Header */}
          <div className="flex items-center justify-between p-4 border-b border-border lg:hidden">
            <h2 className="text-lg font-semibold text-foreground">Menu</h2>
            <button
              onClick={onClose}
              aria-label="Close sidebar"
              className="
                p-2 rounded-lg
                text-foreground hover:bg-underground-lighter
                focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-primary
              "
            >
              <svg aria-hidden="true" className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* Main Navigation */}
          <nav className="flex-1 p-4 space-y-1">
            <div className="mb-4">
              <h3 className="px-2 mb-2 text-xs font-semibold text-foreground-secondary uppercase tracking-wider">
                Discover
              </h3>
              {navItems.map((item) => (
                <Link
                  key={item.path}
                  to={item.path}
                  onClick={handleLinkClick}
                  className={`
                    flex items-center gap-3 px-3 py-2 rounded-lg
                    text-sm font-medium
                    transition-colors
                    ${
                      isActive(item.path)
                        ? 'bg-brand-primary text-white'
                        : 'text-foreground hover:bg-underground-lighter'
                    }
                  `}
                >
                  <span className="text-lg" aria-hidden="true">{item.icon}</span>
                  <span>{item.label}</span>
                </Link>
              ))}
            </div>

            {/* Account Section */}
            {accountItems.length > 0 && (
              <div className="pt-4 border-t border-border">
                <h3 className="px-2 mb-2 text-xs font-semibold text-foreground-secondary uppercase tracking-wider">
                  Account
                </h3>
                {accountItems.map((item) => (
                  <Link
                    key={item.path}
                    to={item.path}
                    onClick={handleLinkClick}
                    className={`
                      flex items-center gap-3 px-3 py-2 rounded-lg
                      text-sm font-medium
                      transition-colors
                      ${
                        isActive(item.path)
                          ? 'bg-brand-primary text-white'
                          : 'text-foreground hover:bg-underground-lighter'
                      }
                    `}
                  >
                    <span className="text-lg" aria-hidden="true">{item.icon}</span>
                    <span>{item.label}</span>
                  </Link>
                ))}
              </div>
            )}

            {/* Featured Scenes (Placeholder) */}
            <div className="pt-4 border-t border-border">
              <h3 className="px-2 mb-2 text-xs font-semibold text-foreground-secondary uppercase tracking-wider">
                Featured Scenes
              </h3>
              <div className="px-2 py-4 text-xs text-foreground-secondary text-center">
                No featured scenes yet
              </div>
            </div>
          </nav>

          {/* Footer */}
          <div className="p-4 border-t border-border">
            <p className="text-xs text-foreground-secondary text-center">
              Subcults v{VERSION}
            </p>
          </div>
        </div>
      </aside>
    </>
  );
}
