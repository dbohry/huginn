// Runs axe-core over every screen, in light and dark, on a phone and on a desktop.
// Usage: BASE=http://127.0.0.1:8080 node a11y.js   (against a fresh, empty data directory)
const { chromium } = require('playwright');
const { start } = require('./site');
const axeSource = require('fs').readFileSync(require.resolve('axe-core/axe.min.js'), 'utf8');

const BASE = process.env.BASE || 'http://127.0.0.1:8080';
const sizes = [
  { name: 'phone', viewport: { width: 390, height: 844 }, touch: true },
  { name: 'desktop', viewport: { width: 1280, height: 800 }, touch: false },
];
const schemes = ['light', 'dark'];
let failed = 0;
let story = 0;
let audited = 0;

async function audit(page, label) {
  await page.evaluate(axeSource);
  const result = await page.evaluate(() => axe.run(document, { runOnly: ['wcag2a', 'wcag2aa', 'wcag21aa', 'best-practice'] }));
  audited++;
  const bad = result.violations;
  console.log((bad.length ? 'FAIL ' : 'PASS ') + label);
  for (const v of bad) {
    failed++;
    console.log(`     ${v.id} (${v.impact}): ${v.help}`);
    for (const n of v.nodes.slice(0, 3)) console.log(`       ${n.target.join(' ')}  ${(n.failureSummary || '').split('\n')[1] || ''}`);
  }
}

const post = (page, path, body) =>
  page.evaluate(([p, b]) => fetch(p, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(b) }).then((r) => r.json()), [path, body]);

(async () => {
  const site = await start();
  const browser = await chromium.launch();

  // Set up the administrator, with feeds and a second user who still has a temporary password.
  const admin = await browser.newContext();
  const p0 = await admin.newPage();
  await p0.goto(BASE + '/');
  await audit(p0, 'setup screen');
  await p0.getByLabel('Username').fill('demo');
  await p0.getByLabel('Password', { exact: true }).fill('demo-password-1');
  await p0.getByLabel('Confirm password').fill('demo-password-1');
  await p0.getByRole('button', { name: 'Create account' }).click();
  await p0.getByRole('dialog', { name: 'Start with a few feeds' }).waitFor();
  await audit(p0, 'first sign-in: starter feeds');
  await p0.getByRole('button', { name: 'Start empty' }).click();
  await p0.getByRole('heading', { name: 'Follow your first feed' }).waitFor();
  await audit(p0, 'empty reader');
  await post(p0, '/api/feeds', { url: site.url + '/fieldnotes.xml' });
  await post(p0, '/api/feeds', { url: site.url + '/systems.atom' });
  await post(p0, '/api/feeds', { url: site.url + '/hostile.xml' });
  await post(p0, '/api/feeds', { url: site.url + '/summaryonly.xml' });
  const created = await post(p0, '/api/users', { username: 'sam', isAdmin: false });
  const cookies = await admin.cookies();
  await admin.close();

  // signed out
  const out = await browser.newContext();
  const po = await out.newPage();
  await po.goto(BASE + '/');
  await audit(po, 'sign-in screen');
  await po.getByLabel('Username').fill('sam');
  await po.getByLabel('Password', { exact: true }).fill(created.temporaryPassword);
  await po.getByRole('button', { name: 'Sign in' }).click();
  await po.waitForURL('**/change-password');
  await audit(po, 'choose a new password');
  await out.close();

  for (const size of sizes) {
    for (const scheme of schemes) {
      const ctx = await browser.newContext({ viewport: size.viewport, hasTouch: size.touch, isMobile: size.touch, colorScheme: scheme });
      await ctx.addCookies(cookies);
      const page = await ctx.newPage();
      const tag = `${size.name}, ${scheme}`;

      await page.goto(BASE + '/');
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
      await page.waitForFunction(() => document.querySelectorAll('.item').length >= 5);
      await audit(page, `${tag}: article list`);

      if (size.name === 'phone') {
        await page.getByRole('button', { name: 'Show feeds' }).click();
        await page.getByRole('navigation', { name: 'Feeds' }).waitFor();
        await audit(page, `${tag}: feed list`);
        await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: /^Field Notes/ }).click();
      } else {
        await audit(page, `${tag}: no article selected`);
        await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: /^Field Notes/ }).click();
      }
      await page.getByRole('heading', { name: 'Field Notes', level: 1 }).waitFor();
      await audit(page, `${tag}: one feed, with its toolbar`);
      await page.getByRole('button', { name: /^Mark all read in Field Notes/ }).click();
      await page.getByText(/marked read in Field Notes|Nothing unread in Field Notes/).waitFor();
      await audit(page, `${tag}: with a message showing`);

      await page.getByRole('button', { name: /A week with only a paper calendar/ }).click();
      await page.getByRole('heading', { name: 'A week with only a paper calendar' }).waitFor();
      await audit(page, `${tag}: article`);

      if (size.name === 'phone') await page.getByRole('button', { name: 'Back to articles' }).click();
      if (size.name === 'phone') await page.getByRole('button', { name: 'Show feeds' }).click();
      await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: 'Add feed' }).click();
      await page.getByRole('dialog').waitFor();
      await page.getByLabel('Feed address').fill(site.url + '/notafeed.html');
      await page.getByRole('dialog').getByRole('button', { name: 'Add feed' }).click();
      await page.getByRole('dialog').getByRole('alert').waitFor();
      await audit(page, `${tag}: add feed dialog with an error`);
      await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click();

      // A story whose feed gives only a summary: the hint, then the full article. Each pass uses its
      // own story, because a loaded article is kept.
      story += 1;
      await page.goto(BASE + '/');
      await page.getByRole('button', { name: new RegExp(`Summary story ${story}\\b`) }).click();
      await page.getByRole('heading', { name: `Summary story ${story}`, level: 2 }).waitFor();
      await page.getByText('This is only the summary the feed provides.').waitFor();
      await audit(page, `${tag}: article with the summary-only hint`);
      await page.getByRole('button', { name: 'Load full article' }).click();
      await page.getByText(/Showing the full article from/).waitFor();
      await audit(page, `${tag}: full article loaded`);

      await page.goto(BASE + '/settings');
      await page.getByRole('heading', { name: 'Users' }).waitFor();
      await audit(page, `${tag}: settings`);
      await ctx.close();
    }
  }

  await browser.close();
  site.close();
  console.log(failed ? `\n${failed} accessibility problems in ${audited} audits` : `\nNo accessibility problems in ${audited} audits`);
  process.exit(failed ? 1 : 0);
})().catch((e) => {
  console.error('FAILED', e);
  process.exit(1);
});
