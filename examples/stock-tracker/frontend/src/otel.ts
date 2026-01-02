import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-web';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { Resource } from '@opentelemetry/resources';
import { SEMRESATTRS_SERVICE_NAME, SEMRESATTRS_SERVICE_VERSION } from '@opentelemetry/semantic-conventions';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request';
import { registerInstrumentations } from '@opentelemetry/instrumentation';

const OTLP_ENDPOINT = import.meta.env.VITE_OTLP_ENDPOINT || 'http://localhost:4318';

export function initOtel() {
  // Create resource with service info
  const resource = new Resource({
    [SEMRESATTRS_SERVICE_NAME]: 'stock-tracker-frontend',
    [SEMRESATTRS_SERVICE_VERSION]: '1.0.0',
    'deployment.environment': 'development',
  });

  // Create OTLP exporter
  const exporter = new OTLPTraceExporter({
    url: `${OTLP_ENDPOINT}/v1/traces`,
  });

  // Create tracer provider
  const provider = new WebTracerProvider({
    resource,
  });

  // Add batch processor
  provider.addSpanProcessor(new BatchSpanProcessor(exporter));

  // Register provider with context manager
  provider.register({
    contextManager: new ZoneContextManager(),
  });

  // Register instrumentations for fetch and XHR
  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/.*/], // Propagate to all URLs
        clearTimingResources: true,
      }),
      new XMLHttpRequestInstrumentation({
        propagateTraceHeaderCorsUrls: [/.*/],
      }),
    ],
  });

  console.log('OpenTelemetry initialized for frontend');
}

