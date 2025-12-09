/**
 * API Client
 * HTTP client with automatic token refresh and retry logic
 */

export interface RequestConfig extends RequestInit {
  skipAuth?: boolean; // Skip adding Authorization header
  skipRetry?: boolean; // Skip retry on 401
}

export interface ApiError {
  status: number;
  code: string;
  message: string;
}

export class ApiClientError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string
  ) {
    super(message);
    this.name = 'ApiClientError';
  }
}

/**
 * Token refresh callback type
 * Should return new access token or null if refresh failed
 */
export type RefreshTokenCallback = () => Promise<string | null>;

/**
 * API Client configuration
 */
interface ApiClientConfig {
  baseURL: string;
  getAccessToken: () => string | null;
  refreshToken: RefreshTokenCallback;
  onUnauthorized: () => void; // Called when refresh fails
}

class ApiClient {
  private config: ApiClientConfig | null = null;
  private refreshPromise: Promise<string | null> | null = null;

  /**
   * Initialize the API client
   * Must be called before making any requests
   */
  initialize(config: ApiClientConfig): void {
    this.config = config;
  }

  /**
   * Make an authenticated API request
   * Automatically adds Authorization header and handles 401 with token refresh
   */
  async request<T>(
    endpoint: string,
    options: RequestConfig = {}
  ): Promise<T> {
    if (!this.config) {
      throw new Error('ApiClient not initialized. Call initialize() first.');
    }

    const { skipAuth, skipRetry, ...fetchOptions } = options;

    // Build full URL
    const url = `${this.config.baseURL}${endpoint}`;

    // Prepare headers
    const headers = new Headers(fetchOptions.headers);
    
    // Add Authorization header if not skipped and token exists
    if (!skipAuth) {
      const token = this.config.getAccessToken();
      if (token) {
        headers.set('Authorization', `Bearer ${token}`);
      }
    }

    // Set default Content-Type for POST/PUT/PATCH if not specified
    if (
      ['POST', 'PUT', 'PATCH'].includes(fetchOptions.method?.toUpperCase() || 'GET') &&
      !headers.has('Content-Type') &&
      fetchOptions.body
    ) {
      headers.set('Content-Type', 'application/json');
    }

    // Make request
    let response = await fetch(url, {
      ...fetchOptions,
      headers,
    });

    // Handle 401 Unauthorized
    if (response.status === 401 && !skipRetry && !skipAuth) {
      // Attempt to refresh token
      const newToken = await this.handleTokenRefresh();

      if (newToken) {
        // Retry original request with new token
        headers.set('Authorization', `Bearer ${newToken}`);
        response = await fetch(url, {
          ...fetchOptions,
          headers,
        });
      } else {
        // Refresh failed, throw error
        const error = await this.parseError(response);
        throw new ApiClientError(error.status, error.code, error.message);
      }
    }

    // Check for error responses
    if (!response.ok) {
      const error = await this.parseError(response);
      throw new ApiClientError(error.status, error.code, error.message);
    }

    // Parse response
    const contentType = response.headers.get('content-type');
    if (contentType?.includes('application/json')) {
      return await response.json();
    }

    // Return empty object for 204 No Content
    if (response.status === 204) {
      return {} as T;
    }

    // For non-JSON responses, return as text
    return (await response.text()) as unknown as T;
  }

  /**
   * Handle token refresh with deduplication
   * Multiple concurrent requests will share the same refresh promise
   */
  private async handleTokenRefresh(): Promise<string | null> {
    if (!this.config) {
      return null;
    }

    // If refresh is already in progress, wait for it
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    // Start new refresh
    this.refreshPromise = this.config.refreshToken();

    try {
      const newToken = await this.refreshPromise;
      
      // Clear refresh promise on success or failure
      this.refreshPromise = null;

      // If refresh failed, notify the app
      if (!newToken) {
        this.config.onUnauthorized();
      }

      return newToken;
    } catch (error) {
      // Clear refresh promise on error
      this.refreshPromise = null;
      
      // Notify the app of unauthorized state
      this.config.onUnauthorized();
      
      return null;
    }
  }

  /**
   * Parse error response
   */
  private async parseError(response: Response): Promise<ApiError> {
    const contentType = response.headers.get('content-type');
    
    if (contentType?.includes('application/json')) {
      try {
        const body = await response.json();
        return {
          status: response.status,
          code: body.code || 'unknown_error',
          message: body.message || response.statusText,
        };
      } catch {
        // Fall through to default error
      }
    }

    return {
      status: response.status,
      code: 'unknown_error',
      message: response.statusText || 'An error occurred',
    };
  }

  /**
   * Convenience methods
   */
  async get<T>(endpoint: string, options?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, { ...options, method: 'GET' });
  }

  async post<T>(endpoint: string, data?: unknown, options?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async put<T>(endpoint: string, data?: unknown, options?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async patch<T>(endpoint: string, data?: unknown, options?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'PATCH',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async delete<T>(endpoint: string, options?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, { ...options, method: 'DELETE' });
  }
}

// Export singleton instance
export const apiClient = new ApiClient();
