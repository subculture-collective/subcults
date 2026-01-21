/**
 * AppLayout Component
 * Main application layout shell with header, navigation, and content outlet
 */

import React, { useState } from 'react';
import { Outlet, Link, useNavigate } from 'react-router-dom';
import { useAuth, authStore } from '../stores/authStore';
import { MiniPlayer } from '../components/MiniPlayer';

// Display constants
const DID_DISPLAY_LENGTH = 20; // Characters to show before truncation
const BREAKPOINT_MOBILE = 767; // Max width for mobile layout
const BREAKPOINT_DESKTOP = 768; // Min width for desktop layout

export const AppLayout: React.FC = () => {
  const { isAuthenticated, isAdmin, user } = useAuth();
  const navigate = useNavigate();
  const [showMobileNav, setShowMobileNav] = useState(false);

  const handleLogout = () => {
    authStore.logout();
    navigate('/');
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100vh',
        width: '100%',
      }}
    >
      {/* Skip to content link for accessibility */}
      <a
        href="#main-content"
        style={{
          position: 'absolute',
          top: '-100px',
          left: '0',
          padding: '0.5rem 1rem',
          backgroundColor: 'white',
          color: '#1a1a1a',
          textDecoration: 'none',
          zIndex: 9999,
        }}
        onFocus={(e) => {
          e.currentTarget.style.top = '0';
        }}
        onBlur={(e) => {
          e.currentTarget.style.top = '-100px';
        }}
      >
        Skip to content
      </a>

      {/* Header */}
      <header
        role="banner"
        style={{
          padding: '1rem',
          backgroundColor: '#1a1a1a',
          color: 'white',
          borderBottom: '1px solid #333',
          flexShrink: 0,
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            maxWidth: '1400px',
            margin: '0 auto',
          }}
        >
          {/* Logo */}
          <Link
            to="/"
            style={{
              fontSize: '1.5rem',
              fontWeight: 'bold',
              color: 'white',
              textDecoration: 'none',
            }}
          >
            Subcults
          </Link>

          {/* Search placeholder */}
          <div
            style={{
              flex: 1,
              maxWidth: '500px',
              margin: '0 2rem',
              display: 'none',
            }}
            className="search-placeholder"
          >
            <input
              type="search"
              placeholder="Search scenes, events..."
              disabled
              title="Search feature coming soon"
              aria-describedby="search-status"
              style={{
                width: '100%',
                padding: '0.5rem',
                borderRadius: '4px',
                border: '1px solid #555',
                backgroundColor: '#2a2a2a',
                color: 'white',
              }}
              aria-label="Search"
            />
            <span
              id="search-status"
              style={{
                position: 'absolute',
                left: '-10000px',
                width: '1px',
                height: '1px',
                overflow: 'hidden',
              }}
            >
              Search feature is not yet implemented
            </span>
          </div>

          {/* Auth status and desktop nav */}
          <nav
            role="navigation"
            aria-label="Main navigation"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '1rem',
            }}
          >
            <div
              style={{
                display: 'none',
              }}
              className="desktop-nav"
            >
              <Link
                to="/settings"
                style={{
                  color: 'white',
                  textDecoration: 'none',
                  padding: '0.5rem',
                }}
              >
                Settings
              </Link>
              {isAdmin && (
                <Link
                  to="/admin"
                  style={{
                    color: 'white',
                    textDecoration: 'none',
                    padding: '0.5rem',
                  }}
                >
                  Admin
                </Link>
              )}
            </div>

            {isAuthenticated ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                <span style={{ fontSize: '0.875rem' }}>
                  {user?.did.slice(0, DID_DISPLAY_LENGTH)}...
                </span>
                <button
                  onClick={handleLogout}
                  style={{
                    padding: '0.5rem 1rem',
                    backgroundColor: '#333',
                    color: 'white',
                    border: 'none',
                    borderRadius: '4px',
                    cursor: 'pointer',
                  }}
                >
                  Logout
                </button>
              </div>
            ) : (
              <Link
                to="/account/login"
                style={{
                  padding: '0.5rem 1rem',
                  backgroundColor: 'white',
                  color: '#1a1a1a',
                  textDecoration: 'none',
                  borderRadius: '4px',
                }}
              >
                Login
              </Link>
            )}

            {/* Mobile menu toggle */}
            <button
              onClick={() => setShowMobileNav(!showMobileNav)}
              style={{
                display: 'block',
                padding: '0.5rem',
                backgroundColor: 'transparent',
                color: 'white',
                border: '1px solid #555',
                borderRadius: '4px',
                cursor: 'pointer',
              }}
              className="mobile-menu-toggle"
              aria-label="Toggle mobile menu"
              aria-expanded={showMobileNav}
            >
              ☰
            </button>
          </nav>
        </div>

        {/* Mobile navigation */}
        {showMobileNav && (
          <nav
            role="navigation"
            aria-label="Mobile navigation"
            style={{
              marginTop: '1rem',
              padding: '1rem',
              backgroundColor: '#2a2a2a',
              borderRadius: '4px',
            }}
            className="mobile-nav"
          >
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '0.5rem',
              }}
            >
              <Link
                to="/"
                style={{
                  color: 'white',
                  textDecoration: 'none',
                  padding: '0.5rem',
                }}
                onClick={() => setShowMobileNav(false)}
              >
                Home
              </Link>
              <Link
                to="/settings"
                style={{
                  color: 'white',
                  textDecoration: 'none',
                  padding: '0.5rem',
                }}
                onClick={() => setShowMobileNav(false)}
              >
                Settings
              </Link>
              {isAdmin && (
                <Link
                  to="/admin"
                  style={{
                    color: 'white',
                    textDecoration: 'none',
                    padding: '0.5rem',
                  }}
                  onClick={() => setShowMobileNav(false)}
                >
                  Admin
                </Link>
              )}
              <Link
                to="/account"
                style={{
                  color: 'white',
                  textDecoration: 'none',
                  padding: '0.5rem',
                }}
                onClick={() => setShowMobileNav(false)}
              >
                Account
              </Link>
            </div>
          </nav>
        )}
      </header>

      {/* Main content */}
      <main
        id="main-content"
        role="main"
        style={{
          flex: 1,
          overflow: 'auto',
          position: 'relative',
        }}
      >
        <Outlet />
      </main>

      {/* Mini Player (persistent across routes) */}
      <MiniPlayer />

      {/* Bottom mobile navigation (optional) */}
      <nav
        role="navigation"
        aria-label="Bottom navigation"
        style={{
          display: 'none',
          padding: '0.75rem',
          backgroundColor: '#1a1a1a',
          borderTop: '1px solid #333',
          flexShrink: 0,
        }}
        className="bottom-nav"
      >
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-around',
            maxWidth: '600px',
            margin: '0 auto',
          }}
        >
          <Link
            to="/"
            style={{
              color: 'white',
              textDecoration: 'none',
              padding: '0.5rem',
              textAlign: 'center',
              fontSize: '0.875rem',
            }}
          >
            Map
          </Link>
          <Link
            to="/account"
            style={{
              color: 'white',
              textDecoration: 'none',
              padding: '0.5rem',
              textAlign: 'center',
              fontSize: '0.875rem',
            }}
          >
            Account
          </Link>
          <Link
            to="/settings"
            style={{
              color: 'white',
              textDecoration: 'none',
              padding: '0.5rem',
              textAlign: 'center',
              fontSize: '0.875rem',
            }}
          >
            Settings
          </Link>
        </div>
      </nav>

      <style>{`
        @media (min-width: ${BREAKPOINT_DESKTOP}px) {
          .desktop-nav {
            display: flex !important;
            align-items: center;
            gap: 1rem;
          }
          .mobile-menu-toggle {
            display: none !important;
          }
          .search-placeholder {
            display: block !important;
          }
        }
        @media (max-width: ${BREAKPOINT_MOBILE}px) {
          .bottom-nav {
            display: block !important;
          }
        }
      `}</style>
    </div>
  );
};
