#!/usr/bin/env bunx
import { chromium } from 'playwright';
import {
  browseAndSearch,
  managePortfolio,
  manageWatchlist,
  triggerErrors,
  chaosKillRestore,
} from './scenarios';
import type { LaunchOptions } from 'playwright';

const FRONTEND_URL = process.env.STOCK_TRACKER_URL || 'http://localhost:3000';
const DEFAULT_DURATION_SECONDS = 120;
const DEFAULT_LOOP_DELAY_SECONDS = 5;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

const log = (...args: unknown[]) => {
  const ts = new Date().toISOString();
  console.log(ts, '-', ...args);
};

type TrafficOptions = {
  users: number;
  duration: number;
  loopDelay: number;
  includeChaos: boolean;
};

function parseArgs(): TrafficOptions {
  const args = process.argv.slice(2);
  const options = {
    users: 1,
    duration: DEFAULT_DURATION_SECONDS,
    loopDelay: DEFAULT_LOOP_DELAY_SECONDS,
    includeChaos: false,
  };

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--users' && args[i + 1]) {
      options.users = Number(args[++i]);
    } else if (arg === '--duration' && args[i + 1]) {
      options.duration = Number(args[++i]);
    } else if (arg === '--loop-delay' && args[i + 1]) {
      options.loopDelay = Number(args[++i]);
    } else if (arg === '--include-chaos') {
      options.includeChaos = true;
    }
  }

  return options;
}

const launchOptions: LaunchOptions = {
  headless: true,
  args: [
    '--no-sandbox',
    '--disable-setuid-sandbox',
    '--disable-dev-shm-usage',
    '--disable-gpu',
  ],
};

async function createPage(browser: Awaited<ReturnType<typeof chromium.launch>>) {
  const context = await browser.newContext();
  const page = await context.newPage();
  log('Navigating to app', FRONTEND_URL);
  await page.goto(FRONTEND_URL, { waitUntil: 'domcontentloaded' });
  return { context, page };
}

async function runUser(id: number, opts: TrafficOptions) {
  log(`User #${id}: launching browser with options`, launchOptions);
  const browser = await chromium.launch(launchOptions);
  let { context, page } = await createPage(browser);
  const start = Date.now();
  const durationMs =
    opts.duration > 0 ? opts.duration * 1000 : DEFAULT_DURATION_SECONDS * 1000;
  let iteration = 0;

  log(`User #${id} started`);

  while (Date.now() - start < durationMs) {
    iteration += 1;
    log(`User #${id} iteration ${iteration} starting`);
    try {
      log(`User #${id} browseAndSearch`);
      await browseAndSearch(page);
      log(`User #${id} managePortfolio`);
      await managePortfolio(page);
      log(`User #${id} manageWatchlist`);
      await manageWatchlist(page);
      log(`User #${id} triggerErrors`);
      await triggerErrors(page);
      if (opts.includeChaos) {
        log(`User #${id} chaosKillRestore`);
        await chaosKillRestore(page);
      }
      await page.waitForTimeout(2000 + Math.random() * 3000);
      log(`User #${id} iteration ${iteration} complete`);
    } catch (err) {
      console.error(`User #${id} iteration ${iteration} error:`, err);
      if (!page.isClosed()) {
        await page.close().catch(() => {});
      }
      if (!context.isClosed()) {
        await context.close().catch(() => {});
      }
      log(`User #${id} recreating browser context after error`);
      ({ context, page } = await createPage(browser));
    }
  }

  await context.close().catch(() => {});
  await browser.close();
  log(`User #${id} finished`);
}

async function main() {
  const opts = parseArgs();
  let loop = 0;

  while (true) {
    loop += 1;
    log(
      `Starting traffic loop #${loop}: ${opts.users} user(s) for ${
        opts.duration
      }s (chaos: ${opts.includeChaos ? 'on' : 'off'})`,
    );

    const loopStart = Date.now();
    const workers = [];

    for (let i = 0; i < opts.users; i++) {
      workers.push(runUser(i + 1, opts));
    }

    try {
      await Promise.all(workers);
    } catch (err) {
      console.error(`Traffic loop #${loop} error (will continue):`, err);
    }
    const elapsed = ((Date.now() - loopStart) / 1000).toFixed(1);
    log(
      `Traffic loop #${loop} complete in ${elapsed}s. Sleeping ${opts.loopDelay}s...`,
    );
    await sleep(opts.loopDelay * 1000);
  }
}

main().catch((err) => {
  console.error('Traffic generator error:', err);
  process.exit(1);
});

