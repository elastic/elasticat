#!/usr/bin/env bunx
import { chromium } from 'playwright';
import {
  browseAndSearch,
  managePortfolio,
  manageWatchlist,
  triggerErrors,
  chaosKillRestore,
} from './scenarios';

const FRONTEND_URL = process.env.STOCK_TRACKER_URL || 'http://localhost:3000';

type TrafficOptions = {
  users: number;
  duration: number;
  includeChaos: boolean;
};

function parseArgs(): TrafficOptions {
  const args = process.argv.slice(2);
  const options = {
    users: 1,
    duration: 120,
    includeChaos: false,
  };

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--users' && args[i + 1]) {
      options.users = Number(args[++i]);
    } else if (arg === '--duration' && args[i + 1]) {
      options.duration = Number(args[++i]);
    } else if (arg === '--include-chaos') {
      options.includeChaos = true;
    }
  }

  return options;
}

async function runUser(id: number, opts: TrafficOptions) {
  const browser = await chromium.launch();
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(FRONTEND_URL, { waitUntil: 'domcontentloaded' });
  const start = Date.now();
  const endTime = opts.duration > 0 ? opts.duration * 1000 : Number.POSITIVE_INFINITY;

  console.log(`User #${id} started`);

  while (Date.now() - start < endTime) {
    await browseAndSearch(page);
    await managePortfolio(page);
    await manageWatchlist(page);
    await triggerErrors(page);
    if (opts.includeChaos) {
      await chaosKillRestore(page);
    }
    await page.waitForTimeout(2000 + Math.random() * 3000);
  }

  await browser.close();
  console.log(`User #${id} finished`);
}

async function main() {
  const opts = parseArgs();
  const workers = [];

  console.log(`Starting traffic generator: ${opts.users} user(s) for ${opts.duration}s`);

  for (let i = 0; i < opts.users; i++) {
    workers.push(runUser(i + 1, opts));
  }

  await Promise.all(workers);
  console.log('Traffic generation complete');
}

main().catch((err) => {
  console.error('Traffic generator error:', err);
  process.exit(1);
});

