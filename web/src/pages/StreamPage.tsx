/**
 * StreamPage Component
 * Live audio streaming room
 * This is lazy-loaded due to heavy dependencies
 */

import React from 'react';
import { useParams } from 'react-router-dom';

export const StreamPage: React.FC = () => {
  const { room } = useParams<{ room: string }>();

  return (
    <div style={{ padding: '2rem' }}>
      <h1>Stream Room</h1>
      <p>Room: {room}</p>
      <p>LiveKit streaming interface will be displayed here.</p>
      <p>This page is lazy-loaded for performance.</p>
    </div>
  );
};
