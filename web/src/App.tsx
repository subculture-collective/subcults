/**
 * App Component
 * Root application component with routing
 */

import { ErrorBoundary } from './components/ErrorBoundary';
import { AppRouter } from './routes';
import './App.css';

function App() {
  return (
    <ErrorBoundary>
      <AppRouter />
    </ErrorBoundary>
  );
}

export default App;

