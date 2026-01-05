import type { Page } from "playwright";

const COMMON_STOCKS = ["AAPL", "GOOG", "MSFT", "NVDA", "TSLA"];

function randomStock() {
  return COMMON_STOCKS[Math.floor(Math.random() * COMMON_STOCKS.length)];
}

async function wait(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export async function browseAndSearch(page: Page) {
  const symbol = randomStock();
  const input = page.locator('form input[type="text"]');
  await input.fill(symbol);
  await page.click('button:has-text("Search")');
  await page.waitForTimeout(1500);
}

export async function managePortfolio(page: Page) {
  await browseAndSearch(page);
  const buyButtons = page.locator('button:has-text("Buy")');
  const count = await buyButtons.count();
  if (count === 0) {
    await page.waitForTimeout(500);
    return;
  }
  const buyButton = buyButtons.first();
  await buyButton.waitFor({ state: "visible", timeout: 15000 });
  const sharesInput = page.locator('input[type="number"]').first();
  await sharesInput.waitFor({ state: "visible", timeout: 5000 });
  await sharesInput.fill(`${Math.floor(Math.random() * 5) + 1}`);
  await buyButton.click();
  await page.waitForTimeout(1000);
}

export async function manageWatchlist(page: Page) {
  await browseAndSearch(page);
  const watchButtons = page.locator('button:has-text("+ Watchlist")');
  if (await watchButtons.count()) {
    const watchButton = watchButtons.first();
    await watchButton.waitFor({ state: "visible", timeout: 15000 });
    await watchButton.click();
    await page.waitForTimeout(500);
  }
  const removeButton = page.locator('button:has-text("Remove")');
  if ((await removeButton.count()) > 0) {
    await removeButton.first().click();
    await page.waitForTimeout(500);
  }
}

export async function triggerErrors(page: Page) {
  for (const failure of ["SLOW", "ERROR", "INVALID"]) {
    const input = page.locator('form input[type="text"]');
    await input.fill(failure);
    await page.click('button:has-text("Search")');
    await page.waitForTimeout(1200);
  }
}

export async function chaosKillRestore(page: Page) {
  await page.click('button:has-text("Kill Stock Service")');
  await wait(2000);
  await page.click('button:has-text("Restore Stock Service")');
  await page.waitForTimeout(1000);
}
