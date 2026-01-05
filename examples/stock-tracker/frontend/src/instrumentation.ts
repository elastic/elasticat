import { trace, context, SpanStatusCode } from '@opentelemetry/api';
import { getTracer, getMeter } from './otel';

/**
 * Wrap a user action in a custom span for tracing
 */
export async function traceUserAction<T>(
  name: string,
  action: () => Promise<T>,
  attributes?: Record<string, string | number | boolean>
): Promise<T> {
  const tracer = getTracer();
  const span = tracer.startSpan(name, {
    kind: 1, // SPAN_KIND_INTERNAL
    attributes: {
      'user.action': name,
      ...attributes,
    },
  });

  try {
    const result = await context.with(trace.setSpan(context.active(), span), async () => {
      return await action();
    });
    span.setStatus({ code: SpanStatusCode.OK });
    return result;
  } catch (error) {
    span.setStatus({
      code: SpanStatusCode.ERROR,
      message: error instanceof Error ? error.message : String(error),
    });
    if (error instanceof Error && error.stack) {
      span.setAttribute('error.stack', error.stack);
    }
    throw error;
  } finally {
    span.end();
  }
}

/**
 * Record a metric value
 */
export function recordMetric(
  name: string,
  value: number,
  attributes?: Record<string, string>
): void {
  try {
    const meter = getMeter();
    const counter = meter.createCounter(name, {
      description: `Custom metric: ${name}`,
    });
    counter.add(value, attributes);
  } catch (error) {
    // Silently fail if metrics aren't initialized
    console.warn('Failed to record metric:', error);
  }
}

/**
 * Capture an error as a span
 */
export function captureError(error: Error, context?: string): void {
  const tracer = getTracer();
  const span = tracer.startSpan('error.captured', {
    kind: 1, // SPAN_KIND_INTERNAL
    attributes: {
      'error.type': 'captured.error',
      'error.message': error.message,
      'error.name': error.name,
    },
  });

  if (context) {
    span.setAttribute('error.context', context);
  }

  if (error.stack) {
    span.setAttribute('error.stack', error.stack);
  }

  span.setStatus({
    code: SpanStatusCode.ERROR,
    message: error.message,
  });

  span.end();
}

