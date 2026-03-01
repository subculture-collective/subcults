/**
 * EventsPage Component
 * Redirects to search results filtered by events type
 */

import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

export function EventsPage() {
  const navigate = useNavigate();

  useEffect(() => {
    navigate('/search?type=events', { replace: true });
  }, [navigate]);

  return null;
}
