// Takes the screenshots used in the README, from a fresh Huginn and the test feeds.
// Usage: BASE=http://127.0.0.1:8080 OUT=../docs/screenshots node screenshots.js
const { chromium } = require('playwright');
const { start } = require('./site');
const path = require('path');

const BASE = process.env.BASE || 'http://127.0.0.1:8080';
const OUT = path.resolve(process.env.OUT || path.join(__dirname, '../docs/screenshots'));

const post = (page, p, body) =>
  page.evaluate(([u, b]) => fetch(u, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(b) }).then((r) => r.json()), [p, body]);

(async () => {
  const site = await start();
  const browser = await chromium.launch();

  const admin = await browser.newContext();
  const p0 = await admin.newPage();
  await p0.goto(BASE + '/');
  await post(p0, '/api/setup', { username: 'demo', password: 'demo-password-1' });
  for (const f of ['/fieldnotes.xml', '/systems.atom', '/gardens.xml']) await post(p0, '/api/feeds', { url: site.url + f });
  const state = await admin.storageState();
  await admin.close();

  for (const scheme of ['light', 'dark']) {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, colorScheme: scheme, storageState: state });
    const page = await ctx.newPage();
    await page.goto(BASE + '/');
    await page.getByRole('button', { name: /A week with only a paper calendar/ }).click();
    await page.getByRole('heading', { name: 'A week with only a paper calendar', level: 2 }).waitFor();
    await page.waitForTimeout(300);
    await page.screenshot({ path: path.join(OUT, `reader-${scheme}.png`) });
    await ctx.close();
  }

  const phone = await browser.newContext({ viewport: { width: 390, height: 780 }, deviceScaleFactor: 2, hasTouch: true, isMobile: true, storageState: state });
  const p = await phone.newPage();
  await p.goto(BASE + '/');
  await p.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
  await p.waitForFunction(() => document.querySelectorAll('.item').length >= 8);
  await p.screenshot({ path: path.join(OUT, 'phone-list.png') });
  await p.getByRole('button', { name: /A week with only a paper calendar/ }).click();
  await p.getByRole('heading', { name: 'A week with only a paper calendar', level: 2 }).waitFor();
  await p.waitForTimeout(300);
  await p.screenshot({ path: path.join(OUT, 'phone-article.png') });

  await browser.close();
  site.close();
  console.log('Screenshots written to ' + OUT);
})().catch((e) => { console.error('FAILED', e); process.exit(1); });
