# Subcults Web Frontend

React + TypeScript + Vite frontend for the Subcults platform.

## Features

- **MapLibre GL** integration with MapTiler tiles
- **Privacy-first** geolocation (opt-in, coarse accuracy)
- Responsive map with ResizeObserver
- TypeScript for type safety
- Vitest for testing

## Setup

1. Install dependencies:
   ```bash
   npm install
   ```

2. Configure environment variables:
   ```bash
   cp .env.example .env
   # Edit .env and add your VITE_MAPTILER_API_KEY
   ```

3. Run development server:
   ```bash
   npm run dev
   ```

## Scripts

- `npm run dev` - Start development server with HMR
- `npm run build` - Build for production
- `npm run preview` - Preview production build locally
- `npm run test` - Run unit tests
- `npm run test:ui` - Run tests with Vitest UI
- `npm run lint` - Run ESLint

## MapView Component

The `MapView` component is a privacy-conscious wrapper around MapLibre GL:

```tsx
import { useRef } from 'react';
import { MapView, type MapViewHandle } from './components/MapView';

function App() {
  const mapRef = useRef<MapViewHandle>(null);

  const flyToLocation = () => {
    mapRef.current?.flyTo([-122.4194, 37.7749], 14);
  };

  return (
    <MapView
      ref={mapRef}
      enableGeolocation={false}  // Privacy: opt-in only
      onLoad={(map) => console.log('Map loaded')}
    />
  );
}
```

### Props

- `apiKey?: string` - MapTiler API key (or use VITE_MAPTILER_API_KEY env var)
- `initialPosition?: { bounds?, center?, zoom? }` - Initial map position
- `enableGeolocation?: boolean` - Enable geolocation fallback (default: false)
- `className?: string` - CSS class for map container
- `onLoad?: (map) => void` - Callback when map loads
- `onGeolocationSuccess?: (position) => void` - Geolocation success callback
- `onGeolocationError?: (error) => void` - Geolocation error callback

### Imperative Methods (via ref)

- `getMap()` - Get underlying MapLibre map instance
- `flyTo(center, zoom?)` - Animate to location
- `getBounds()` - Get current map bounds

## Privacy Considerations

- Geolocation is **opt-in** via `enableGeolocation` prop
- When enabled, uses `enableHighAccuracy: false` for coarse location
- MapTiler API key is client-side (acceptable for public tile access)
- Document key rotation procedure in production

## Testing

Tests use Vitest with React Testing Library:

```bash
npm test
```

All MapView functionality is tested including:
- Map initialization
- Resize handling
- Imperative ref methods
- Geolocation privacy controls
- Component lifecycle (mount/unmount)

## Build Output

Production build creates optimized static assets in `dist/`:

```bash
npm run build
```

The build output is ready to be served by Caddy, nginx, or any static file server.
