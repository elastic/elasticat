import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import { initOtel } from './otel';

// Initialize OpenTelemetry before rendering
initOtel();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);

