import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-web';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { Resource } from '@opentelemetry/resources';
import { SEMRESATTRS_SERVICE_NAME, SEMRESATTRS_SERVICE_VERSION } from '@opentelemetry/semantic-conventions';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';
import { LongTaskInstrumentation } from '@opentelemetry/instrumentation-long-task';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { MeterProvider, PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-http';
import { trace, context, SpanStatusCode } from '@opentelemetry/api';

const OTLP_ENDPOINT = import.meta.env.VITE_OTLP_ENDPOINT || 'http://localhost:4318';

let tracerProvider: WebTracerProvider | null = null;
let meterProvider: MeterProvider | null = null;

export function initOtel() {
  // Create resource with service info
  const resource = new Resource({
    [SEMRESATTRS_SERVICE_NAME]: 'stock-tracker-frontend',
    [SEMRESATTRS_SERVICE_VERSION]: '1.0.0',
    'deployment.environment': 'development',
  });

  // Create OTLP exporters
  const traceExporter = new OTLPTraceExporter({
    url: `${OTLP_ENDPOINT}/v1/traces`,
  });

  const metricExporter = new OTLPMetricExporter({
    url: `${OTLP_ENDPOINT}/v1/metrics`,
  });

  // Create tracer provider
  tracerProvider = new WebTracerProvider({
    resource,
  });

  // Add batch processor for traces
  tracerProvider.addSpanProcessor(new BatchSpanProcessor(traceExporter));

  // Register provider with context manager
  tracerProvider.register({
    contextManager: new ZoneContextManager(),
  });

  // Create meter provider for metrics
  meterProvider = new MeterProvider({
    resource,
    readers: [
      new PeriodicExportingMetricReader({
        exporter: metricExporter,
        exportIntervalMillis: 10000, // Export every 10 seconds
      }),
    ],
  });

  // Register instrumentations
  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/.*/], // Propagate to all URLs
        clearTimingResources: true,
      }),
      new XMLHttpRequestInstrumentation({
        propagateTraceHeaderCorsUrls: [/.*/],
      }),
      new DocumentLoadInstrumentation(),
      new UserInteractionInstrumentation({
        enabled: (event) => {
          // Only track clicks and keydown events to avoid noise
          return event.type === 'click' || event.type === 'keydown';
        },
      }),
      new LongTaskInstrumentation(),
    ],
  });

  // Setup global error handlers
  setupErrorHandlers();

  console.log('OpenTelemetry initialized for frontend');
}

function setupErrorHandlers() {
  const tracer = trace.getTracer('stock-tracker-frontend', '1.0.0');

  // Handle uncaught JavaScript errors
  window.addEventListener('error', (event) => {
    const span = tracer.startSpan('javascript.error', {
      kind: 1, // SPAN_KIND_INTERNAL
      attributes: {
        'error.type': 'javascript.error',
        'error.message': event.message,
        'error.filename': event.filename || 'unknown',
        'error.lineno': event.lineno || 0,
        'error.colno': event.colno || 0,
      },
    });

    if (event.error?.stack) {
      span.setAttribute('error.stack', event.error.stack);
    }

    span.setStatus({
      code: SpanStatusCode.ERROR,
      message: event.message,
    });

    span.end();
  });

  // Handle unhandled promise rejections
  window.addEventListener('unhandledrejection', (event) => {
    const span = tracer.startSpan('javascript.unhandled_rejection', {
      kind: 1, // SPAN_KIND_INTERNAL
      attributes: {
        'error.type': 'unhandled.rejection',
      },
    });

    if (event.reason instanceof Error) {
      span.setAttribute('error.message', event.reason.message);
      if (event.reason.stack) {
        span.setAttribute('error.stack', event.reason.stack);
      }
    } else {
      span.setAttribute('error.message', String(event.reason));
    }

    span.setStatus({
      code: SpanStatusCode.ERROR,
      message: String(event.reason),
    });

    span.end();
  });
}

export function getTracer() {
  return trace.getTracer('stock-tracker-frontend', '1.0.0');
}

export function getMeter() {
  if (!meterProvider) {
    throw new Error('MeterProvider not initialized. Call initOtel() first.');
  }
  return meterProvider.getMeter('stock-tracker-frontend', '1.0.0');
}
