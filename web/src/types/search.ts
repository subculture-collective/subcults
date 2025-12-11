/**
 * Search Types
 * Type definitions for global search functionality
 */

import { Scene, Event } from './scene';

/**
 * Post entity (simplified for search results)
 * Full post type definition can be expanded as needed
 */
export interface Post {
  id: string;
  title?: string;
  content: string;
  author_did?: string;
  scene_id?: string;
  event_id?: string;
  created_at?: string;
  record_did?: string;
  record_rkey?: string;
}

/**
 * Search result types for each entity
 */
export type SceneSearchResult = Scene;
export type EventSearchResult = Event;
export type PostSearchResult = Post;

/**
 * Grouped search results
 */
export interface SearchResults {
  scenes: SceneSearchResult[];
  events: EventSearchResult[];
  posts: PostSearchResult[];
}

/**
 * Search result item with type discriminator
 */
export type SearchResultItem = 
  | { type: 'scene'; data: SceneSearchResult }
  | { type: 'event'; data: EventSearchResult }
  | { type: 'post'; data: PostSearchResult };

/**
 * Search query parameters
 */
export interface SearchParams {
  query: string;
  limit?: number;
}

/**
 * Search state for hook
 */
export interface SearchState {
  results: SearchResults;
  loading: boolean;
  error: string | null;
}
