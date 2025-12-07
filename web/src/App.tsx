import { useRef } from 'react';
import { MapView, type MapViewHandle } from './components/MapView';
import './App.css';

function App() {
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

  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <div style={{ padding: '1rem', background: '#1a1a1a', color: 'white' }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem' }}>Subcults Map</h1>
        <div style={{ marginTop: '0.5rem' }}>
          <button onClick={handleFlyToSF} style={{ marginRight: '0.5rem' }}>
            Fly to SF
          </button>
          <button onClick={handleFlyToNYC}>Fly to NYC</button>
        </div>
      </div>
      <div style={{ flex: 1 }}>
        <MapView
          ref={mapRef}
          enableGeolocation={false}
          onLoad={(map) => {
            console.log('Map loaded:', map);
          }}
        />
      </div>
    </div>
  );
}

export default App;

