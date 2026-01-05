/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string;
  readonly VITE_OTLP_ENDPOINT: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}


