import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import { initOtel } from './otel';
import { initWebVitals } from './webvitals';

// Initialize OpenTelemetry before rendering
initOtel();

// Initialize Web Vitals collection
initWebVitals();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);

