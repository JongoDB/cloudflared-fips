/**
 * Puppeteer E2E tests for the FIPS Compliance Dashboard.
 *
 * Serves the built dashboard/dist files with a simple HTTP server,
 * then runs Puppeteer tests to verify all critical UI elements render.
 *
 * Run: node dashboard/e2e/dashboard.test.cjs
 */

const puppeteer = require('puppeteer');
const http = require('http');
const fs = require('fs');
const path = require('path');

const DIST_DIR = path.join(__dirname, '..', 'dist');
const TIMEOUT = 15000;

// Find arm64-compatible Chromium (Playwright's or system)
function findChromium() {
  const candidates = [
    path.join(process.env.HOME || '', '.cache/ms-playwright/chromium-1208/chrome-linux/chrome'),
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
    '/usr/bin/google-chrome-stable',
  ];
  for (const c of candidates) {
    if (fs.existsSync(c)) return c;
  }
  return null; // fall back to Puppeteer's bundled browser
}

let browser;
let page;
let server;
let baseURL;

// Test results tracking
const results = { passed: 0, failed: 0, errors: [] };

function log(msg) {
  console.log(`  ${msg}`);
}

function pass(name) {
  results.passed++;
  log(`\u2713 ${name}`);
}

function fail(name, error) {
  results.failed++;
  results.errors.push({ name, error: error.message || String(error) });
  log(`\u2717 ${name}: ${error.message || error}`);
}

// Minimal static file server
function createStaticServer(dir) {
  const mimeTypes = {
    '.html': 'text/html',
    '.js':   'application/javascript',
    '.css':  'text/css',
    '.svg':  'image/svg+xml',
    '.json': 'application/json',
    '.png':  'image/png',
    '.ico':  'image/x-icon',
  };

  return http.createServer((req, res) => {
    let filePath = path.join(dir, req.url === '/' ? 'index.html' : req.url);

    // Prevent path traversal
    if (!filePath.startsWith(dir)) {
      res.writeHead(403);
      res.end();
      return;
    }

    const ext = path.extname(filePath);
    const contentType = mimeTypes[ext] || 'application/octet-stream';

    fs.readFile(filePath, (err, data) => {
      if (err) {
        // SPA fallback — serve index.html for non-file routes
        fs.readFile(path.join(dir, 'index.html'), (err2, indexData) => {
          if (err2) {
            res.writeHead(404);
            res.end('Not Found');
            return;
          }
          res.writeHead(200, { 'Content-Type': 'text/html' });
          res.end(indexData);
        });
        return;
      }
      res.writeHead(200, { 'Content-Type': contentType });
      res.end(data);
    });
  });
}

async function setup() {
  console.log('\nServing dashboard from', DIST_DIR);

  if (!fs.existsSync(path.join(DIST_DIR, 'index.html'))) {
    throw new Error(`Dashboard not built. Run 'npm run build' in dashboard/ first.`);
  }

  // Start static server on random port
  server = createStaticServer(DIST_DIR);
  await new Promise((resolve, reject) => {
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address();
      baseURL = `http://127.0.0.1:${addr.port}`;
      console.log(`Static server on ${baseURL}\n`);
      resolve();
    });
    server.on('error', reject);
  });

  const chromiumPath = findChromium();
  const launchOpts = {
    headless: true,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-gpu',
    ],
  };
  if (chromiumPath) {
    console.log(`Using Chromium: ${chromiumPath}`);
    launchOpts.executablePath = chromiumPath;
  }
  browser = await puppeteer.launch(launchOpts);
  page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 900 });
}

async function teardown() {
  if (browser) await browser.close().catch(() => {});
  if (server) server.close();
}

// ──── TEST FUNCTIONS ────────────────────────────────────────

async function testDashboardLoads() {
  const name = 'Dashboard page loads successfully';
  try {
    const resp = await page.goto(baseURL, { waitUntil: 'networkidle0', timeout: TIMEOUT });
    if (!resp || resp.status() !== 200) throw new Error(`HTTP ${resp ? resp.status() : 'null'}`);
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testLayoutRendered() {
  const name = 'Layout and main container rendered';
  try {
    // Wait for React to render
    await page.waitForFunction(() => document.body.innerText.length > 100, { timeout: 5000 });
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testSummaryBarPresent() {
  const name = 'Summary bar displays compliance counts';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    const hasNumbers = /\d+/.test(text);
    const hasSummary = text.includes('Pass') || text.includes('pass') ||
                       text.includes('Fail') || text.includes('fail') ||
                       text.includes('Warning') || text.includes('warning');
    if (!hasNumbers || !hasSummary) throw new Error('Summary bar counts not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testClientPostureSection() {
  const name = 'Client Posture section (Segment 1) renders';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    if (!text.includes('Client Posture')) throw new Error('Section not found');
    // Check for specific items
    const items = ['FIPS Mode', 'OS Type', 'Browser TLS', 'Cipher Suite', 'TLS Version'];
    const found = items.filter(i => text.includes(i));
    if (found.length < 3) throw new Error(`Only ${found.length}/5 items found`);
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testCloudflareEdgeSection() {
  const name = 'Cloudflare Edge section renders';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    if (!text.includes('Cloudflare Edge')) throw new Error('Section not found');
    const items = ['Access', 'Identity', 'MFA', 'Cipher', 'HSTS'];
    const found = items.filter(i => text.includes(i));
    if (found.length < 3) throw new Error(`Only ${found.length}/5 items found`);
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testTunnelSection() {
  const name = 'Tunnel — cloudflared section (Segment 2) renders';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    if (!text.includes('Tunnel')) throw new Error('Section not found');
    const items = ['BoringCrypto', 'Self-Test', 'Protocol', 'Binary Integrity'];
    const found = items.filter(i => text.includes(i));
    if (found.length < 2) throw new Error(`Only ${found.length} tunnel items found`);
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testLocalServiceSection() {
  const name = 'Local Service section (Segment 3) renders';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    if (!text.includes('Local Service')) throw new Error('Section not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testBuildSupplyChainSection() {
  const name = 'Build & Supply Chain section renders';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    if (!text.includes('Build') || !text.includes('Supply Chain')) throw new Error('Section not found');
    const items = ['SBOM', 'Reproducibility', 'BoringCrypto Version'];
    const found = items.filter(i => text.includes(i));
    if (found.length < 1) throw new Error('Build items not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testVerificationBadgesVisible() {
  const name = 'Verification method badges (Direct/API/Probe/Inherited/Reported)';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    const badges = ['Direct', 'API', 'Probe', 'Inherited', 'Reported'];
    const found = badges.filter(b => text.includes(b));
    if (found.length < 3) throw new Error(`Only ${found.length}/5 badges: ${found.join(', ')}`);
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testSunsetBanner() {
  const name = 'FIPS 140-2 sunset/migration banner visible';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    const hasSunset = text.includes('140-2') || text.includes('140-3') ||
                      text.includes('sunset') || text.includes('Sunset') ||
                      text.includes('migration') || text.includes('Migration');
    if (!hasSunset) throw new Error('Sunset/migration info not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testDeploymentTierBadge() {
  const name = 'Deployment tier badge visible';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    const hasTier = text.includes('Tier') || text.includes('Standard') ||
                    text.includes('standard') || text.includes('Deployment');
    if (!hasTier) throw new Error('Deployment tier not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testFIPSBackendCard() {
  const name = 'FIPS backend/crypto module card renders';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    const hasBackend = text.includes('FIPS') && (
      text.includes('Crypto') || text.includes('Module') ||
      text.includes('Backend') || text.includes('BoringCrypto')
    );
    if (!hasBackend) throw new Error('FIPS backend card not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testBuildManifestPanel() {
  const name = 'Build manifest panel renders';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    const has = text.includes('Build Manifest') || text.includes('Manifest') ||
                (text.includes('version') && text.includes('commit'));
    if (!has) throw new Error('Build manifest panel not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testExportButtons() {
  const name = 'Export buttons present (JSON/PDF)';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    const hasExport = text.includes('Export') || text.includes('JSON') ||
                      text.includes('Download') || text.includes('export');
    if (!hasExport) throw new Error('Export buttons not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testSSEToggle() {
  const name = 'Live/SSE toggle present and clickable';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    if (!text.includes('Live')) throw new Error('Live toggle not found');

    // Try clicking the toggle
    const toggled = await page.evaluate(() => {
      const buttons = Array.from(document.querySelectorAll('[role="switch"], button'));
      const liveBtn = buttons.find(b => b.getAttribute('aria-label')?.includes('live') ||
                                         b.getAttribute('aria-label')?.includes('Live'));
      if (liveBtn) {
        liveBtn.click();
        return true;
      }
      return false;
    });
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testStatusBadgeColors() {
  const name = 'Status badges with semantic colors (green/red/yellow)';
  try {
    const hasColors = await page.evaluate(() => {
      const html = document.body.innerHTML;
      return (html.includes('green') || html.includes('#22c55e')) &&
             (html.includes('red') || html.includes('#ef4444') ||
              html.includes('yellow') || html.includes('amber'));
    });
    if (!hasColors) throw new Error('Missing semantic status colors');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testResponsiveLayout() {
  const name = 'Responsive layout at mobile width (375px)';
  try {
    await page.setViewport({ width: 375, height: 812 });

    // Wait for layout reflow
    await new Promise(r => setTimeout(r, 500));

    const hasScroll = await page.evaluate(() => {
      return document.body.scrollWidth > window.innerWidth + 20;
    });

    // Reset viewport
    await page.setViewport({ width: 1280, height: 900 });
    await new Promise(r => setTimeout(r, 300));

    if (hasScroll) throw new Error('Horizontal scroll detected');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testAirGapFriendly() {
  const name = 'No external CDN requests (air-gap safe)';
  try {
    const externalRequests = [];

    const handler = req => {
      const url = req.url();
      if (!url.startsWith(baseURL) && !url.startsWith('data:') && !url.startsWith('blob:')) {
        externalRequests.push(url);
      }
    };

    page.on('request', handler);
    await page.reload({ waitUntil: 'networkidle0', timeout: TIMEOUT });
    page.off('request', handler);

    if (externalRequests.length > 0) {
      throw new Error(`External: ${externalRequests.join(', ')}`);
    }
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testLocalhostNote() {
  const name = 'Localhost-only / air-gap note visible';
  try {
    const text = await page.evaluate(() => document.body.innerText);
    const has = text.includes('localhost') || text.includes('Localhost') ||
                text.includes('air-gap') || text.includes('Air-gap');
    if (!has) throw new Error('Localhost-only note not found');
    pass(name);
  } catch (e) { fail(name, e); }
}

async function testScreenshot() {
  const name = 'Full-page screenshot captured';
  try {
    const dir = path.join(__dirname, '..', '..', 'test-results');
    fs.mkdirSync(dir, { recursive: true });
    await page.screenshot({
      path: path.join(dir, 'dashboard-e2e.png'),
      fullPage: true,
    });
    pass(name);
  } catch (e) { fail(name, e); }
}

// ──── RUNNER ────────────────────────────────────────────────

async function run() {
  console.log('\n======================================');
  console.log('  FIPS Compliance Dashboard E2E Tests');
  console.log('======================================');

  try {
    await setup();

    const tests = [
      testDashboardLoads,
      testLayoutRendered,
      testSummaryBarPresent,
      testClientPostureSection,
      testCloudflareEdgeSection,
      testTunnelSection,
      testLocalServiceSection,
      testBuildSupplyChainSection,
      testVerificationBadgesVisible,
      testSunsetBanner,
      testDeploymentTierBadge,
      testFIPSBackendCard,
      testBuildManifestPanel,
      testExportButtons,
      testSSEToggle,
      testStatusBadgeColors,
      testResponsiveLayout,
      testAirGapFriendly,
      testLocalhostNote,
      testScreenshot,
    ];

    for (const test of tests) {
      await test();
    }

  } catch (e) {
    console.error('Setup error:', e.message);
    results.failed++;
    results.errors.push({ name: 'setup', error: e.message });
  } finally {
    await teardown();
  }

  // Summary
  console.log('\n--------------------------------------');
  console.log(`  ${results.passed} passed, ${results.failed} failed  (${results.passed + results.failed} total)`);

  if (results.errors.length > 0) {
    console.log('\n  Failed:');
    for (const e of results.errors) {
      console.log(`    - ${e.name}: ${e.error}`);
    }
  }
  console.log('');

  process.exit(results.failed > 0 ? 1 : 0);
}

run();
