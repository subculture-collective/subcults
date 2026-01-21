/**
 * App Component
 * Root application component with routing
 */

import { useEffect } from 'react';
import { ErrorBoundary } from './components/ErrorBoundary';
import { ToastContainer } from './components/ToastContainer';
import { ThemeProvider } from './components/ThemeProvider';
import { AppRouter } from './routes';
import { authStore } from './stores/authStore';
import { useStreamingStore } from './stores/streamingStore';
import './App.css';

function App() {
  // Initialize auth on app startup
  useEffect(() => {
    authStore.initialize();
  }, []);

  // Initialize streaming store on app startup
  useEffect(() => {
    const streamingStore = useStreamingStore.getState();
    streamingStore.initialize();
  }, []);

  return (
    <ThemeProvider>
      <ErrorBoundary>
        <AppRouter />
        <ToastContainer />
      </ErrorBoundary>
    </ThemeProvider>
  );
}

export default App;

