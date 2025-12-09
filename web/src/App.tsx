/**
 * App Component
 * Root application component with routing
 */

import { useEffect } from 'react';
import { ErrorBoundary } from './components/ErrorBoundary';
import { AppRouter } from './routes';
import { authStore } from './stores/authStore';
import './App.css';

function App() {
  // Initialize auth on app startup
  useEffect(() => {
    authStore.initialize();
  }, []);

  return (
    <ErrorBoundary>
      <AppRouter />
    </ErrorBoundary>
  );
}

export default App;

