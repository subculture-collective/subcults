/**
 * HomePage Component
 * Main map view showing scenes and events
 */

import React, { useRef } from 'react';
import { MapView, type MapViewHandle } from '../components/MapView';

export const HomePage: React.FC = () => {
  const mapRef = useRef<MapViewHandle>(null);

  return (
    <div style={{ height: '100%', width: '100%' }}>
      <MapView
        ref={mapRef}
        enableGeolocation={false}
        onLoad={(map) => {
          console.log('Map loaded:', map);
        }}
      />
    </div>
  );
};
