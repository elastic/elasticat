import { onCLS, onFID, onLCP, Metric } from 'web-vitals';
import { getMeter } from './otel';

export function initWebVitals() {
  let meter;
  try {
    meter = getMeter();
  } catch (error) {
    console.warn('Web Vitals: MeterProvider not initialized, skipping Web Vitals collection');
    return;
  }

  // Create counters/histograms for Web Vitals
  const lcpCounter = meter.createHistogram('web.vitals.lcp', {
    description: 'Largest Contentful Paint (LCP)',
    unit: 'ms',
  });

  const fidCounter = meter.createHistogram('web.vitals.fid', {
    description: 'First Input Delay (FID)',
    unit: 'ms',
  });

  const clsCounter = meter.createHistogram('web.vitals.cls', {
    description: 'Cumulative Layout Shift (CLS)',
    unit: '1',
  });

  // Helper to record a metric
  const recordMetric = (metric: Metric, histogram: ReturnType<typeof meter.createHistogram>) => {
    histogram.record(metric.value, {
      id: metric.id,
      name: metric.name,
      rating: metric.rating,
      navigationType: metric.navigationType || 'unknown',
    });
  };

  // Collect LCP (Largest Contentful Paint)
  onLCP((metric) => {
    recordMetric(metric, lcpCounter);
    console.log('LCP:', metric.value, 'ms');
  });

  // Collect FID (First Input Delay)
  onFID((metric) => {
    recordMetric(metric, fidCounter);
    console.log('FID:', metric.value, 'ms');
  });

  // Collect CLS (Cumulative Layout Shift)
  onCLS((metric) => {
    recordMetric(metric, clsCounter);
    console.log('CLS:', metric.value);
  });

  console.log('Web Vitals collection initialized');
}

