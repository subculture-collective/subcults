/**
 * API Client tests
 * Tests for HTTP client with automatic token refresh
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { apiClient, ApiClientError } from './api-client';

describe('ApiClient', () => {
  const mockBaseURL = 'http://localhost:3000';
  let mockGetAccessToken: ReturnType<typeof vi.fn>;
  let mockRefreshToken: ReturnType<typeof vi.fn>;
  let mockOnUnauthorized: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    // Reset mocks
    mockGetAccessToken = vi.fn();
    mockRefreshToken = vi.fn();
    mockOnUnauthorized = vi.fn();

    // Initialize API client with mocks
    apiClient.initialize({
      baseURL: mockBaseURL,
      getAccessToken: mockGetAccessToken,
      refreshToken: mockRefreshToken,
      onUnauthorized: mockOnUnauthorized,
    });

    // Clear fetch mocks
    vi.clearAllMocks();
  });

  describe('request', () => {
    it('adds Authorization header when token is available', async () => {
      const mockToken = 'mock-access-token';
      mockGetAccessToken.mockReturnValue(mockToken);

      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ data: 'success' }),
      });

      await apiClient.get('/test');

      expect(global.fetch).toHaveBeenCalledWith(
        `${mockBaseURL}/test`,
        expect.objectContaining({
          headers: expect.any(Headers),
        })
      );

      const call = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
      const headers = call[1].headers as Headers;
      expect(headers.get('Authorization')).toBe(`Bearer ${mockToken}`);
    });

    it('does not add Authorization header when skipAuth is true', async () => {
      mockGetAccessToken.mockReturnValue('mock-token');

      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ data: 'success' }),
      });

      await apiClient.get('/test', { skipAuth: true });

      const call = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
      const headers = call[1].headers as Headers;
      expect(headers.get('Authorization')).toBeNull();
    });

    it('refreshes token and retries on 401 response', async () => {
      const oldToken = 'old-token';
      const newToken = 'new-token';
      
      mockGetAccessToken.mockReturnValue(oldToken);
      mockRefreshToken.mockResolvedValue(newToken);

      // First call returns 401, second call succeeds
      global.fetch = vi
        .fn()
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ code: 'unauthorized', message: 'Token expired' }),
        })
        .mockResolvedValueOnce({
          ok: true,
          status: 200,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ data: 'success' }),
        });

      const result = await apiClient.get<{ data: string }>('/test');

      expect(mockRefreshToken).toHaveBeenCalledTimes(1);
      expect(global.fetch).toHaveBeenCalledTimes(2);
      expect(result).toEqual({ data: 'success' });

      // Second call should use new token
      const secondCall = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[1];
      const headers = secondCall[1].headers as Headers;
      expect(headers.get('Authorization')).toBe(`Bearer ${newToken}`);
    });

    it('calls onUnauthorized when refresh fails', async () => {
      mockGetAccessToken.mockReturnValue('old-token');
      mockRefreshToken.mockResolvedValue(null);

      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ code: 'unauthorized', message: 'Token expired' }),
      });

      await expect(apiClient.get('/test')).rejects.toThrow(ApiClientError);
      expect(mockRefreshToken).toHaveBeenCalledTimes(1);
      expect(mockOnUnauthorized).toHaveBeenCalledTimes(1);
    });

    it('does not retry when skipRetry is true', async () => {
      mockGetAccessToken.mockReturnValue('token');

      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ code: 'unauthorized', message: 'Token expired' }),
      });

      await expect(apiClient.get('/test', { skipRetry: true })).rejects.toThrow(ApiClientError);
      expect(mockRefreshToken).not.toHaveBeenCalled();
      expect(global.fetch).toHaveBeenCalledTimes(1);
    });

    it('deduplicates concurrent refresh requests', async () => {
      mockGetAccessToken.mockReturnValue('old-token');
      mockRefreshToken.mockImplementation(
        () => new Promise(resolve => setTimeout(() => resolve('new-token'), 100))
      );

      // All requests return 401
      global.fetch = vi
        .fn()
        .mockResolvedValue({
          ok: false,
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ code: 'unauthorized', message: 'Token expired' }),
        })
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ code: 'unauthorized', message: 'Token expired' }),
        })
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ code: 'unauthorized', message: 'Token expired' }),
        })
        // Then all retries succeed
        .mockResolvedValue({
          ok: true,
          status: 200,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ data: 'success' }),
        });

      // Make 3 concurrent requests
      const requests = [
        apiClient.get('/test1'),
        apiClient.get('/test2'),
        apiClient.get('/test3'),
      ];

      await Promise.all(requests);

      // Refresh should only be called once
      expect(mockRefreshToken).toHaveBeenCalledTimes(1);
    });

    it('throws ApiClientError on non-401 error responses', async () => {
      mockGetAccessToken.mockReturnValue('token');

      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ code: 'not_found', message: 'Resource not found' }),
      });

      await expect(apiClient.get('/test')).rejects.toThrow(ApiClientError);
      await expect(apiClient.get('/test')).rejects.toMatchObject({
        status: 404,
        code: 'not_found',
        message: 'Resource not found',
      });
    });

    it('returns empty object for 204 No Content', async () => {
      mockGetAccessToken.mockReturnValue('token');

      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 204,
        headers: new Headers(),
      });

      const result = await apiClient.delete('/test');
      expect(result).toEqual({});
    });
  });

  describe('convenience methods', () => {
    beforeEach(() => {
      mockGetAccessToken.mockReturnValue('token');
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ data: 'success' }),
      });
    });

    it('get method makes GET request', async () => {
      await apiClient.get('/test');

      expect(global.fetch).toHaveBeenCalledWith(
        `${mockBaseURL}/test`,
        expect.objectContaining({ method: 'GET' })
      );
    });

    it('post method makes POST request with JSON body', async () => {
      const data = { name: 'test' };
      await apiClient.post('/test', data);

      expect(global.fetch).toHaveBeenCalledWith(
        `${mockBaseURL}/test`,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(data),
        })
      );

      const call = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
      const headers = call[1].headers as Headers;
      expect(headers.get('Content-Type')).toBe('application/json');
    });

    it('put method makes PUT request with JSON body', async () => {
      const data = { name: 'test' };
      await apiClient.put('/test', data);

      expect(global.fetch).toHaveBeenCalledWith(
        `${mockBaseURL}/test`,
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify(data),
        })
      );
    });

    it('patch method makes PATCH request with JSON body', async () => {
      const data = { name: 'test' };
      await apiClient.patch('/test', data);

      expect(global.fetch).toHaveBeenCalledWith(
        `${mockBaseURL}/test`,
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify(data),
        })
      );
    });

    it('delete method makes DELETE request', async () => {
      await apiClient.delete('/test');

      expect(global.fetch).toHaveBeenCalledWith(
        `${mockBaseURL}/test`,
        expect.objectContaining({ method: 'DELETE' })
      );
    });
  });
});
