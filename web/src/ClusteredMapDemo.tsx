import { useRef } from 'react';
import { ClusteredMapView } from './components/ClusteredMapView';
import type { MapViewHandle } from './components/MapView';
import './App.css';

/**
 * Demo application showing ClusteredMapView usage
 * 
 * To use this demo:
 * 1. Replace App.tsx content with this file
 * 2. Set VITE_MAPTILER_API_KEY in your .env file
 * 3. Ensure backend API is running with /api/scenes and /api/events endpoints
 * 4. Run `npm run dev`
 */
function ClusteredMapDemo() {
  const mapRef = useRef<MapViewHandle>(null);

  const handleFlyToSF = () => {
    if (mapRef.current) {
      mapRef.current.flyTo([-122.4194, 37.7749], 14);
    }
  };

  const handleFlyToNYC = () => {
    if (mapRef.current) {
      mapRef.current.flyTo([-74.006, 40.7128], 14);
    }
  };

  const handleFlyToLA = () => {
    if (mapRef.current) {
      mapRef.current.flyTo([-118.2437, 34.0522], 14);
    }
  };

  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <div style={{ padding: '1rem', background: '#1a1a1a', color: 'white' }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem' }}>
          Subcults - Clustered Scene & Event Map
        </h1>
        <p style={{ margin: '0.5rem 0', fontSize: '0.875rem', opacity: 0.8 }}>
          Interactive map with privacy-first clustering of underground music scenes and events
        </p>
        <div style={{ marginTop: '0.5rem' }}>
          <button onClick={handleFlyToSF} style={{ marginRight: '0.5rem' }}>
            📍 San Francisco
          </button>
          <button onClick={handleFlyToNYC} style={{ marginRight: '0.5rem' }}>
            📍 New York
          </button>
          <button onClick={handleFlyToLA}>
            📍 Los Angeles
          </button>
        </div>
        <div style={{ marginTop: '0.5rem', fontSize: '0.75rem', opacity: 0.6 }}>
          💡 Tip: Click clusters to expand • Blue = Scenes • Pink = Events
        </div>
      </div>
      <div style={{ flex: 1, position: 'relative' }}>
        <ClusteredMapView
          ref={mapRef}
          enableGeolocation={false}
          onLoad={(map) => {
            console.log('Clustered map loaded:', map);
            console.log('Clustering configuration:', {
              clusterRadius: 50,
              clusterMaxZoom: 14,
              privacyEnforced: true,
            });
          }}
          initialPosition={{
            center: [-122.4194, 37.7749],
            zoom: 12,
          }}
        />
      </div>
    </div>
  );
}

export default ClusteredMapDemo;
