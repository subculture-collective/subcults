/**
 * ScenesPage Component
 * Redirects to search results filtered by scenes type
 */

import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

export function ScenesPage() {
  const navigate = useNavigate();

  useEffect(() => {
    navigate('/search?type=scenes', { replace: true });
  }, [navigate]);

  return null;
}
