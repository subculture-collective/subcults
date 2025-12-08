import { useEffect, useRef, useCallback } from 'react';
import type { Scene, Event } from '../types/scene';

/**
 * Props for DetailPanel component
 */
export interface DetailPanelProps {
  /**
   * Whether the panel is open
   */
  isOpen: boolean;
  
  /**
   * Handler called when panel should close
   */
  onClose: () => void;
  
  /**
   * The scene or event to display
   */
  entity: Scene | Event | null;
  
  /**
   * Whether the panel is loading data
   */
  loading?: boolean;
  
  /**
   * Optional analytics callback for tracking panel events
   */
  onAnalyticsEvent?: (eventName: string, data?: Record<string, unknown>) => void;
}

/**
 * DetailPanel - Sliding side panel for scene/event details
 * 
 * Features:
 * - Accessible keyboard focus trap
 * - ESC key to close
 * - Backdrop click to close
 * - Slide-in animation
 * - Privacy-first (no precise coords without consent)
 * 
 * Accessibility:
 * - role="dialog"
 * - aria-modal="true"
 * - aria-labelledby for title
 * - Focus trapped within panel when open
 * - Returns focus to previously focused element on close
 */
export function DetailPanel({
  isOpen,
  onClose,
  entity,
  loading = false,
  onAnalyticsEvent,
}: DetailPanelProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  // Track focus before panel opens
  useEffect(() => {
    if (isOpen) {
      previousFocusRef.current = document.activeElement as HTMLElement;
      
      // Emit analytics event
      if (onAnalyticsEvent && entity) {
        onAnalyticsEvent('detail_panel_open', {
          entity_type: 'scene_id' in entity ? 'event' : 'scene',
          entity_id: entity.id,
        });
      }
      
      // Focus close button after animation
      setTimeout(() => {
        closeButtonRef.current?.focus();
      }, 300);
    } else if (previousFocusRef.current) {
      // Return focus when closing
      previousFocusRef.current.focus();
      
      // Emit analytics event
      if (onAnalyticsEvent && entity) {
        onAnalyticsEvent('detail_panel_close', {
          entity_type: 'scene_id' in entity ? 'event' : 'scene',
          entity_id: entity.id,
        });
      }
    }
  }, [isOpen, entity, onAnalyticsEvent]);

  // Handle ESC key
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape' && isOpen) {
      e.preventDefault();
      onClose();
    }
  }, [isOpen, onClose]);

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  // Focus trap
  useEffect(() => {
    if (!isOpen || !panelRef.current) return;

    const panel = panelRef.current;
    const focusableElements = panel.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    
    if (focusableElements.length === 0) return;

    const firstElement = focusableElements[0];
    const lastElement = focusableElements[focusableElements.length - 1];

    const handleTabKey = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return;

      if (e.shiftKey) {
        // Shift+Tab
        if (document.activeElement === firstElement) {
          e.preventDefault();
          lastElement.focus();
        }
      } else {
        // Tab
        if (document.activeElement === lastElement) {
          e.preventDefault();
          firstElement.focus();
        }
      }
    };

    panel.addEventListener('keydown', handleTabKey as EventListener);
    return () => panel.removeEventListener('keydown', handleTabKey as EventListener);
  }, [isOpen]);

  // Prevent body scroll when panel is open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    
    return () => {
      document.body.style.overflow = '';
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const isScene = entity && !('scene_id' in entity);
  const entityType = isScene ? 'scene' : 'event';

  return (
    <>
      {/* Backdrop */}
      <div
        className="detail-panel-backdrop"
        onClick={onClose}
        aria-hidden="true"
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.5)',
          zIndex: 999,
          animation: 'fadeIn 0.3s ease-out',
        }}
      />
      
      {/* Panel */}
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="detail-panel-title"
        className="detail-panel"
        style={{
          position: 'fixed',
          top: 0,
          right: 0,
          bottom: 0,
          width: '100%',
          maxWidth: '400px',
          backgroundColor: '#1a1a1a',
          color: '#fff',
          zIndex: 1000,
          overflowY: 'auto',
          boxShadow: '-4px 0 16px rgba(0, 0, 0, 0.3)',
          animation: 'slideInRight 0.3s ease-out',
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: '1.5rem',
            borderBottom: '1px solid #333',
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
          }}
        >
          <div style={{ flex: 1 }}>
            <div
              style={{
                fontSize: '0.75rem',
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
                color: isScene ? '#11b4da' : '#f28cb1',
                marginBottom: '0.5rem',
                fontWeight: 600,
              }}
            >
              {entityType}
            </div>
            {loading ? (
              <div style={{ fontSize: '1.5rem', fontWeight: 600 }}>Loading...</div>
            ) : (
              <h2
                id="detail-panel-title"
                style={{
                  margin: 0,
                  fontSize: '1.5rem',
                  fontWeight: 600,
                  lineHeight: 1.3,
                }}
              >
                {entity?.name || 'Unknown'}
              </h2>
            )}
          </div>
          
          <button
            ref={closeButtonRef}
            onClick={onClose}
            aria-label="Close detail panel"
            style={{
              marginLeft: '1rem',
              padding: '0.5rem',
              background: 'transparent',
              border: 'none',
              color: '#fff',
              cursor: 'pointer',
              fontSize: '1.5rem',
              lineHeight: 1,
              opacity: 0.7,
              transition: 'opacity 0.2s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.opacity = '1';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.opacity = '0.7';
            }}
          >
            ×
          </button>
        </div>

        {/* Content */}
        {loading ? (
          <div style={{ padding: '1.5rem', textAlign: 'center', color: '#999' }}>
            Loading details...
          </div>
        ) : entity ? (
          <div style={{ padding: '1.5rem' }}>
            {/* Description */}
            {entity.description && (
              <div style={{ marginBottom: '1.5rem' }}>
                <p style={{ margin: 0, lineHeight: 1.6, color: '#ccc' }}>
                  {entity.description}
                </p>
              </div>
            )}

            {/* Tags */}
            {isScene && (entity as Scene).tags && (entity as Scene).tags!.length > 0 && (
              <div style={{ marginBottom: '1.5rem' }}>
                <h3
                  style={{
                    margin: '0 0 0.75rem 0',
                    fontSize: '0.875rem',
                    fontWeight: 600,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    color: '#999',
                  }}
                >
                  Tags
                </h3>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                  {(entity as Scene).tags!.map((tag) => (
                    <span
                      key={tag}
                      style={{
                        padding: '0.25rem 0.75rem',
                        backgroundColor: '#333',
                        borderRadius: '12px',
                        fontSize: '0.875rem',
                        color: '#ccc',
                      }}
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {/* Location Privacy Notice */}
            <div
              style={{
                marginTop: '1.5rem',
                padding: '1rem',
                backgroundColor: '#2a2a2a',
                borderRadius: '8px',
                fontSize: '0.875rem',
                color: '#999',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <span>📍</span>
                <span>
                  {entity.allow_precise
                    ? 'Precise location shared'
                    : 'Approximate location (privacy preserved)'}
                </span>
              </div>
            </div>

            {/* Placeholder for future features */}
            <div
              style={{
                marginTop: '1.5rem',
                padding: '1rem',
                backgroundColor: '#2a2a2a',
                borderRadius: '8px',
                fontSize: '0.875rem',
                color: '#999',
                textAlign: 'center',
              }}
            >
              <p style={{ margin: 0 }}>
                More features coming soon:
                <br />
                • Trust score
                <br />
                • Upcoming events
                <br />
                • Join stream
                <br />
                • View posts
              </p>
            </div>
          </div>
        ) : null}
      </div>

      <style>{`
        @keyframes fadeIn {
          from { opacity: 0; }
          to { opacity: 1; }
        }
        
        @keyframes slideInRight {
          from {
            transform: translateX(100%);
          }
          to {
            transform: translateX(0);
          }
        }
      `}</style>
    </>
  );
}
