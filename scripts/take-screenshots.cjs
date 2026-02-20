#!/usr/bin/env node
// Take screenshots of the FIPS compliance dashboard for documentation.
// Requires: Puppeteer installed, Vite dev server running on port 5173.
// Uses Playwright's Chromium binary for ARM64 compatibility.

const puppeteer = require('puppeteer-core');
const path = require('path');
const os = require('os');
const fs = require('fs');

const BASE_URL = 'http://localhost:5173';
const OUT_DIR = 'docs/screenshots';

function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function findChromium() {
  // Try Playwright's Chromium first
  const pwBase = path.join(os.homedir(), '.cache', 'ms-playwright');
  if (fs.existsSync(pwBase)) {
    const dirs = fs.readdirSync(pwBase).filter(d => d.startsWith('chromium-')).sort().reverse();
    for (const dir of dirs) {
      const bin = path.join(pwBase, dir, 'chrome-linux', 'chrome');
      if (fs.existsSync(bin)) return bin;
    }
  }
  // Fallback to system chromium
  for (const p of ['/usr/bin/chromium', '/usr/bin/chromium-browser', '/usr/bin/google-chrome']) {
    if (fs.existsSync(p)) return p;
  }
  throw new Error('No Chromium binary found. Install via: npx playwright install chromium');
}

async function main() {
  const executablePath = findChromium();
  console.log(`Using Chromium: ${executablePath}`);

  const browser = await puppeteer.launch({
    executablePath,
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu'],
  });

  const page = await browser.newPage();

  // ── Screenshot 1: Full dashboard overview ──
  console.log('1/5 Dashboard overview...');
  await page.setViewport({ width: 1440, height: 900 });
  await page.goto(BASE_URL, { waitUntil: 'networkidle0' });
  await page.waitForSelector('h1');
  await delay(500);
  await page.screenshot({
    path: `${OUT_DIR}/dashboard-overview.png`,
    fullPage: false,
  });

  // ── Screenshot 2: Full page (all sections visible) ──
  console.log('2/5 Full page...');
  await page.screenshot({
    path: `${OUT_DIR}/dashboard-full.png`,
    fullPage: true,
  });

  // ── Screenshot 3: Expand a checklist item to show details ──
  console.log('3/5 Expanded checklist item...');
  const tunnelItems = await page.$$('button');
  let clicked = false;
  for (const btn of tunnelItems) {
    const text = await btn.evaluate(el => el.textContent);
    if (text && text.includes('BoringCrypto Active')) {
      await btn.click();
      clicked = true;
      break;
    }
  }
  if (!clicked) {
    for (const btn of tunnelItems) {
      const text = await btn.evaluate(el => el.textContent);
      if (text && text.includes('Pass') && text.includes('critical')) {
        await btn.click();
        break;
      }
    }
  }
  await delay(300);
  // Scroll to the expanded item
  await page.evaluate(() => {
    const dts = document.querySelectorAll('dt');
    for (const dt of dts) {
      if (dt.textContent.includes('What')) {
        dt.scrollIntoView({ block: 'center' });
        break;
      }
    }
  });
  await delay(200);
  await page.screenshot({
    path: `${OUT_DIR}/checklist-item-expanded.png`,
    fullPage: false,
  });

  // ── Screenshot 4: Build Manifest panel expanded ──
  console.log('4/5 Build manifest panel...');
  await page.goto(BASE_URL, { waitUntil: 'networkidle0' });
  await delay(500);
  const buttons = await page.$$('button');
  for (const btn of buttons) {
    const text = await btn.evaluate(el => el.textContent);
    if (text && text.includes('Build Manifest')) {
      await btn.click();
      break;
    }
  }
  await delay(300);
  // Scroll manifest into view
  await page.evaluate(() => {
    const h4s = document.querySelectorAll('h4');
    for (const h4 of h4s) {
      if (h4.textContent.includes('Build Info')) {
        h4.scrollIntoView({ block: 'start' });
        break;
      }
    }
  });
  await delay(200);
  await page.screenshot({
    path: `${OUT_DIR}/build-manifest-expanded.png`,
    fullPage: false,
  });

  // ── Screenshot 5: Summary bar close-up ──
  console.log('5/5 Summary bar...');
  await page.goto(BASE_URL, { waitUntil: 'networkidle0' });
  await delay(500);
  await page.setViewport({ width: 1440, height: 500 });
  await page.screenshot({
    path: `${OUT_DIR}/summary-bar.png`,
    fullPage: false,
  });

  await browser.close();
  console.log(`Done. Screenshots saved to ${OUT_DIR}/`);
}

main().catch(err => {
  console.error('Screenshot failed:', err);
  process.exit(1);
});
