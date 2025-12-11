/**
 * SearchBar Component
 * Global search bar with typeahead suggestions, keyboard navigation, and ARIA compliance
 */

import { useState, useRef, useEffect, KeyboardEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSearch } from '../hooks/useSearch';
import type { SearchResultItem } from '../types/search';

// Display constants
const POST_TITLE_TRUNCATE_LENGTH = 50;
const SECONDARY_INFO_TRUNCATE_LENGTH = 60;

export interface SearchBarProps {
  /**
   * Placeholder text for the input field
   */
  placeholder?: string;
  /**
   * Additional CSS classes
   */
  className?: string;
  /**
   * Callback when a result is selected
   */
  onSelect?: (item: SearchResultItem) => void;
  /**
   * Auto-focus on mount (default: false)
   */
  autoFocus?: boolean;
}

/**
 * SearchBar provides unified search across scenes, events, and posts
 * with typeahead suggestions, keyboard navigation, and accessibility
 */
export function SearchBar({
  placeholder = 'Search scenes, events, posts...',
  className = '',
  onSelect,
  autoFocus = false,
}: SearchBarProps) {
  const navigate = useNavigate();
  const { results, loading, error, search, clear } = useSearch();

  const [inputValue, setInputValue] = useState('');
  const [isOpen, setIsOpen] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);

  const inputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Flatten results into a single array for keyboard navigation
  const flatResults: SearchResultItem[] = [
    ...results.scenes.map((scene) => ({ type: 'scene' as const, data: scene })),
    ...results.events.map((event) => ({ type: 'event' as const, data: event })),
    ...results.posts.map((post) => ({ type: 'post' as const, data: post })),
  ];

  const hasResults = flatResults.length > 0;

  /**
   * Handle input change
   */
  const handleInputChange = (value: string) => {
    setInputValue(value);
    setSelectedIndex(-1);
    search(value);

    if (value.trim()) {
      setIsOpen(true);
    } else {
      setIsOpen(false);
    }
  };

  /**
   * Clear input and results
   */
  const handleClear = () => {
    setInputValue('');
    setIsOpen(false);
    setSelectedIndex(-1);
    clear();
    inputRef.current?.focus();
  };

  /**
   * Navigate to result
   */
  const navigateToResult = (item: SearchResultItem) => {
    if (onSelect) {
      onSelect(item);
    }

    // Navigate based on type
    switch (item.type) {
      case 'scene':
        navigate(`/scenes/${item.data.id}`);
        break;
      case 'event':
        navigate(`/events/${item.data.id}`);
        break;
      case 'post':
        navigate(`/posts/${item.data.id}`);
        break;
    }

    // Close dropdown and clear
    handleClear();
  };

  /**
   * Handle keyboard navigation
   */
  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (!isOpen) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex((prev) =>
          prev < flatResults.length - 1 ? prev + 1 : prev
        );
        break;

      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex((prev) => (prev > 0 ? prev - 1 : -1));
        break;

      case 'Enter':
        e.preventDefault();
        if (selectedIndex >= 0 && flatResults[selectedIndex]) {
          navigateToResult(flatResults[selectedIndex]);
        }
        break;

      case 'Escape':
        e.preventDefault();
        setIsOpen(false);
        setSelectedIndex(-1);
        break;
    }
  };

  /**
   * Close dropdown on outside click
   */
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node) &&
        !inputRef.current?.contains(event.target as Node)
      ) {
        setIsOpen(false);
        setSelectedIndex(-1);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  /**
   * Scroll selected item into view
   */
  useEffect(() => {
    if (selectedIndex >= 0 && dropdownRef.current) {
      const selectedElement = dropdownRef.current.querySelector(
        `[data-index="${selectedIndex}"]`
      );
      // Check if scrollIntoView exists (not available in some test environments)
      if (selectedElement && typeof selectedElement.scrollIntoView === 'function') {
        selectedElement.scrollIntoView({ block: 'nearest' });
      }
    }
  }, [selectedIndex]);

  /**
   * Get icon for result type
   */
  const getIcon = (type: SearchResultItem['type']) => {
    switch (type) {
      case 'scene':
        return '🎭';
      case 'event':
        return '📅';
      case 'post':
        return '📝';
    }
  };

  /**
   * Get display name for result
   */
  const getDisplayName = (item: SearchResultItem) => {
    switch (item.type) {
      case 'scene':
        return item.data.name;
      case 'event':
        return item.data.name;
      case 'post':
        return item.data.title || item.data.content?.substring(0, POST_TITLE_TRUNCATE_LENGTH) || 'Untitled Post';
    }
  };

  /**
   * Get secondary info for result
   */
  const getSecondaryInfo = (item: SearchResultItem) => {
    switch (item.type) {
      case 'scene':
        return item.data.description?.substring(0, SECONDARY_INFO_TRUNCATE_LENGTH) || null;
      case 'event':
        return item.data.description?.substring(0, SECONDARY_INFO_TRUNCATE_LENGTH) || null;
      case 'post':
        return item.data.content?.substring(0, SECONDARY_INFO_TRUNCATE_LENGTH) || null;
    }
  };

  return (
    <div className={`relative ${className}`}>
      {/* Search Input */}
      <div className="relative">
        <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
          <span className="text-foreground-tertiary" aria-hidden="true">
            🔍
          </span>
        </div>

        <input
          ref={inputRef}
          type="text"
          role="combobox"
          aria-expanded={isOpen}
          aria-controls="search-results"
          aria-autocomplete="list"
          aria-activedescendant={
            selectedIndex >= 0 ? `search-result-${selectedIndex}` : undefined
          }
          value={inputValue}
          onChange={(e) => handleInputChange(e.target.value)}
          onKeyDown={handleKeyDown}
          onFocus={() => inputValue.trim() && setIsOpen(true)}
          placeholder={placeholder}
          autoFocus={autoFocus}
          className="
            w-full pl-10 pr-10 py-2 rounded-lg
            bg-background-secondary border border-border
            text-foreground placeholder:text-foreground-tertiary
            focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-primary
            focus:border-brand-primary
            theme-transition
          "
        />

        {/* Clear Button */}
        {inputValue && (
          <button
            onClick={handleClear}
            aria-label="Clear search"
            className="
              absolute inset-y-0 right-0 pr-3 flex items-center
              text-foreground-tertiary hover:text-foreground
              focus:outline-none focus-visible:text-brand-primary
            "
          >
            <span aria-hidden="true">✕</span>
          </button>
        )}
      </div>

      {/* Results Dropdown */}
      {isOpen && (
        <div
          ref={dropdownRef}
          id="search-results"
          role="listbox"
          className="
            absolute z-50 w-full mt-2 rounded-lg
            bg-background-secondary border border-border
            shadow-lg max-h-96 overflow-y-auto
          "
        >
          {/* Loading State */}
          {loading && (
            <div className="p-4 text-center">
              <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-brand-primary border-t-transparent" />
              <p className="mt-2 text-sm text-foreground-secondary">Searching...</p>
            </div>
          )}

          {/* Error State */}
          {error && !loading && (
            <div className="p-4 text-center">
              <p className="text-sm text-red-500">{error}</p>
            </div>
          )}

          {/* Empty State */}
          {!loading && !error && !hasResults && inputValue.trim() && (
            <div className="p-4 text-center">
              <p className="text-sm text-foreground-tertiary">
                No results found for "{inputValue}"
              </p>
            </div>
          )}

          {/* Results */}
          {!loading && !error && hasResults && (
            <>
              {/* Scenes */}
              {results.scenes.length > 0 && (
                <div>
                  <div className="px-3 py-2 text-xs font-semibold text-foreground-tertiary uppercase tracking-wider bg-background">
                    Scenes
                  </div>
                  {results.scenes.map((scene, idx) => {
                    const flatIdx = idx;
                    const item: SearchResultItem = { type: 'scene', data: scene };
                    return (
                      <button
                        key={scene.id}
                        id={`search-result-${flatIdx}`}
                        data-index={flatIdx}
                        role="option"
                        aria-selected={selectedIndex === flatIdx}
                        onClick={() => navigateToResult(item)}
                        className={`
                          w-full px-3 py-2 text-left flex items-start gap-3
                          hover:bg-underground-lighter
                          focus:outline-none focus-visible:bg-underground-lighter
                          ${selectedIndex === flatIdx ? 'bg-underground-lighter' : ''}
                        `}
                      >
                        <span className="text-xl flex-shrink-0" aria-hidden="true">
                          {getIcon('scene')}
                        </span>
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-foreground truncate">
                            {getDisplayName(item)}
                          </div>
                          {getSecondaryInfo(item) && (
                            <div className="text-xs text-foreground-secondary truncate">
                              {getSecondaryInfo(item)}
                            </div>
                          )}
                        </div>
                      </button>
                    );
                  })}
                </div>
              )}

              {/* Events */}
              {results.events.length > 0 && (
                <div>
                  <div className="px-3 py-2 text-xs font-semibold text-foreground-tertiary uppercase tracking-wider bg-background">
                    Events
                  </div>
                  {results.events.map((event, idx) => {
                    const flatIdx = results.scenes.length + idx;
                    const item: SearchResultItem = { type: 'event', data: event };
                    return (
                      <button
                        key={event.id}
                        id={`search-result-${flatIdx}`}
                        data-index={flatIdx}
                        role="option"
                        aria-selected={selectedIndex === flatIdx}
                        onClick={() => navigateToResult(item)}
                        className={`
                          w-full px-3 py-2 text-left flex items-start gap-3
                          hover:bg-underground-lighter
                          focus:outline-none focus-visible:bg-underground-lighter
                          ${selectedIndex === flatIdx ? 'bg-underground-lighter' : ''}
                        `}
                      >
                        <span className="text-xl flex-shrink-0" aria-hidden="true">
                          {getIcon('event')}
                        </span>
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-foreground truncate">
                            {getDisplayName(item)}
                          </div>
                          {getSecondaryInfo(item) && (
                            <div className="text-xs text-foreground-secondary truncate">
                              {getSecondaryInfo(item)}
                            </div>
                          )}
                        </div>
                      </button>
                    );
                  })}
                </div>
              )}

              {/* Posts */}
              {results.posts.length > 0 && (
                <div>
                  <div className="px-3 py-2 text-xs font-semibold text-foreground-tertiary uppercase tracking-wider bg-background">
                    Posts
                  </div>
                  {results.posts.map((post, idx) => {
                    const flatIdx = results.scenes.length + results.events.length + idx;
                    const item: SearchResultItem = { type: 'post', data: post };
                    return (
                      <button
                        key={post.id}
                        id={`search-result-${flatIdx}`}
                        data-index={flatIdx}
                        role="option"
                        aria-selected={selectedIndex === flatIdx}
                        onClick={() => navigateToResult(item)}
                        className={`
                          w-full px-3 py-2 text-left flex items-start gap-3
                          hover:bg-underground-lighter
                          focus:outline-none focus-visible:bg-underground-lighter
                          ${selectedIndex === flatIdx ? 'bg-underground-lighter' : ''}
                        `}
                      >
                        <span className="text-xl flex-shrink-0" aria-hidden="true">
                          {getIcon('post')}
                        </span>
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-foreground truncate">
                            {getDisplayName(item)}
                          </div>
                          {getSecondaryInfo(item) && (
                            <div className="text-xs text-foreground-secondary truncate">
                              {getSecondaryInfo(item)}
                            </div>
                          )}
                        </div>
                      </button>
                    );
                  })}
                </div>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}
