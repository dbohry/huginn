// The main journeys, in a real browser, against a fresh Huginn with an empty data directory.
// Usage: BASE=http://127.0.0.1:8080 node flows.js
const { chromium } = require('playwright');
const { start } = require('./site');
const assert = require('assert');

const BASE = process.env.BASE || 'http://127.0.0.1:8080';
let passed = 0;

async function step(name, fn) {
  try {
    await fn();
    passed++;
    console.log('PASS ' + name);
  } catch (e) {
    console.log('FAIL ' + name + '\n     ' + String(e.message).split('\n').join('\n     '));
    process.exitCode = 1;
    throw e;
  }
}

const unreadOf = async (page, feed) => {
  const row = page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: new RegExp('^' + feed) });
  const badge = row.locator('.count');
  return (await badge.count()) === 0 ? 0 : Number((await badge.innerText()).trim()); // a zero is not shown
};

async function addFeed(page, address) {
  await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: 'Add feed' }).click();
  await page.getByLabel('Feed address').fill(address);
  await page.getByRole('dialog').getByRole('button', { name: 'Add feed' }).click();
}

(async () => {
  const site = await start();
  const browser = await chromium.launch();
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();
  const errors = [];
  page.on('pageerror', (e) => errors.push(e.message));
  page.on('dialog', (d) => d.accept());

  try {
    await step('first run: create the administrator', async () => {
      await page.goto(BASE + '/');
      await page.waitForURL('**/setup');
      await page.getByLabel('Username').fill('demo');
      await page.getByLabel('Password', { exact: true }).fill('demo-password-1');
      await page.getByLabel('Confirm password').fill('demo-password-1');
      await page.getByRole('button', { name: 'Create account' }).click();
    });

    await step('the first sign-in offers starter feeds by topic, and "Start empty" declines them', async () => {
      const dialog = page.getByRole('dialog', { name: 'Start with a few feeds' });
      await dialog.waitFor();
      const topics = dialog.getByRole('group', { name: 'Topics' }).getByRole('button');
      assert.ok((await topics.count()) >= 5, 'several topics');
      // The first two topics are chosen, with a few feeds ticked.
      assert.strictEqual(await dialog.getByRole('button', { name: /^Technology/ }).getAttribute('aria-pressed'), 'true');
      assert.ok((await dialog.getByRole('checkbox', { checked: true }).count()) >= 4);
      // Choosing another topic shows its feeds; choosing it again hides them.
      await dialog.getByRole('button', { name: /^Fashion/ }).click();
      await dialog.getByText('Vogue', { exact: true }).waitFor();
      await dialog.getByRole('button', { name: /^Fashion/ }).click();
      assert.strictEqual(await dialog.getByText('Vogue', { exact: true }).count(), 0);
      await dialog.getByRole('button', { name: 'Start empty' }).click();
      await page.getByRole('heading', { name: 'Follow your first feed' }).waitFor();
    });

    await step('the picker is offered only once, and can be opened again by hand', async () => {
      await page.reload();
      await page.getByRole('heading', { name: 'Follow your first feed' }).waitFor();
      assert.strictEqual(await page.getByRole('dialog').count(), 0, 'no picker on the next visit');
      await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: 'Starter feeds' }).click();
      const dialog = page.getByRole('dialog', { name: 'Start with a few feeds' });
      await dialog.waitFor();
      assert.strictEqual(await dialog.getByRole('button', { name: 'Cancel' }).count(), 1);
      await dialog.getByRole('button', { name: 'Cancel' }).click();
    });

    await step('adding something that is not a feed explains why', async () => {
      await addFeed(page, site.url + '/notafeed.html');
      await page.getByRole('dialog').getByRole('alert').waitFor();
      assert.match(await page.getByRole('dialog').getByRole('alert').innerText(), /not an RSS, Atom or JSON feed/);
      await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click();
    });

    await step("adding a site's home page finds its feed", async () => {
      await addFeed(page, site.url + '/systems/');
      await page.getByRole('heading', { name: 'Systems Weekly', level: 1 }).waitFor();
      assert.strictEqual(await unreadOf(page, 'Systems Weekly'), 2);
    });

    await step('adding a feed twice is refused', async () => {
      await addFeed(page, site.url + '/systems.atom');
      await page.getByRole('dialog').getByText('You already follow that feed.').waitFor();
      await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click();
    });

    await step('adding an RSS feed directly', async () => {
      await addFeed(page, site.url + '/fieldnotes.xml');
      await page.getByRole('heading', { name: 'Field Notes', level: 1 }).waitFor();
      assert.strictEqual(await unreadOf(page, 'Field Notes'), 3);
      assert.strictEqual(await unreadOf(page, 'All items'), 5);
    });

    await step('reading an article marks it read, and it can be marked unread again', async () => {
      await page.getByRole('button', { name: /A week with only a paper calendar/ }).click();
      await page.getByRole('heading', { name: 'A week with only a paper calendar', level: 2 }).waitFor();
      assert.match(await page.locator('.content').innerText(), /What I noticed/);
      await page.getByRole('button', { name: 'Mark unread' }).waitFor();
      assert.strictEqual(await unreadOf(page, 'Field Notes'), 2);
      await page.getByRole('button', { name: 'Mark unread' }).click();
      await page.getByRole('button', { name: 'Mark read' }).waitFor();
      assert.strictEqual(await unreadOf(page, 'Field Notes'), 3);
    });

    await step('"Unread only" hides what has been read', async () => {
      await page.getByRole('button', { name: /Walking the long way/ }).click();
      await page.getByRole('heading', { name: 'Walking the long way to the station', level: 2 }).waitFor();
      await page.getByLabel('Unread only').check();
      await page.waitForFunction(() => document.querySelectorAll('.item').length === 2);
      const titles = (await page.locator('.item .title').allInnerTexts()).sort();
      assert.deepStrictEqual(titles, ['A week with only a paper calendar', 'Notes on keeping a notebook']);
      await page.getByLabel('Unread only').uncheck();
      await page.waitForFunction(() => document.querySelectorAll('.item').length === 3);
    });

    await step('keyboard shortcuts move through the list and toggle read, but not while typing', async () => {
      // Walking the long way is open and read; the feed is Field Notes, newest first.
      await page.keyboard.press('j');
      await page.getByRole('heading', { name: 'Notes on keeping a notebook', level: 2 }).waitFor();
      await page.keyboard.press('k');
      await page.getByRole('heading', { name: 'Walking the long way to the station', level: 2 }).waitFor();
      await page.keyboard.press('m');
      await page.getByRole('button', { name: 'Mark read' }).waitFor();
      await page.keyboard.press('m');
      await page.getByRole('button', { name: 'Mark unread' }).waitFor();
      // Typing in a field is not a shortcut.
      await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: 'Add feed' }).click();
      await page.getByLabel('Feed address').pressSequentially('jkm');
      assert.strictEqual(await page.getByLabel('Feed address').inputValue(), 'jkm');
      await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click();
      await page.getByRole('heading', { name: 'Walking the long way to the station', level: 2 }).waitFor();
    });

    await step('"Mark all read" clears the counts for the feed', async () => {
      await page.getByRole('button', { name: 'Mark all read' }).click();
      await page.getByText(/\d+ articles? marked read in Field Notes/).waitFor(); // and says so
      await page.waitForFunction(() => [...document.querySelectorAll('.item .dot.unread')].length === 0);
      assert.strictEqual(await unreadOf(page, 'Field Notes'), 0);
      assert.strictEqual(await unreadOf(page, 'Systems Weekly'), 2);
    });

    await step('an article from a hostile feed cannot run code or show unsafe content', async () => {
      await addFeed(page, site.url + '/hostile.xml');
      await page.getByRole('button', { name: /Totally harmless post/ }).click();
      await page.getByRole('heading', { name: 'Totally harmless post', level: 2 }).waitFor();
      await page.waitForTimeout(500);
      const html = await page.locator('.content').innerHTML();
      for (const bad of ['<script', 'onclick', 'onerror', 'javascript:', '<iframe', '<style']) {
        assert.ok(!html.toLowerCase().includes(bad), `content still has ${bad}: ${html}`);
      }
      assert.match(await page.locator('.content').innerText(), /Visible text/);
      assert.strictEqual(await page.evaluate(() => window.__pwned), undefined, 'feed code ran');
      const fine = page.locator('.content a', { hasText: 'a fine link' });
      assert.strictEqual(await fine.getAttribute('target'), '_blank');
      assert.match(await fine.getAttribute('rel'), /noopener/);
      assert.strictEqual(await page.getByRole('link', { name: 'Open original' }).count(), 0, 'a javascript: link was offered');
    });

    await step('a long list scrolls inside its pane, never the page', async () => {
      await addFeed(page, site.url + '/many.xml');
      await page.getByRole('heading', { name: 'Long Feed', level: 1 }).waitFor();
      await page.waitForFunction(() => document.querySelectorAll('.item').length >= 50);
      const pageHeights = () => page.evaluate(() => ({ doc: document.documentElement.scrollHeight, view: innerHeight, y: scrollY }));
      let h = await pageHeights();
      assert.ok(h.doc <= h.view, `the page is ${h.doc}px tall in a ${h.view}px window: hidden text is stretching it`);
      // Scroll the list to its end and then keep going: the page must stay put.
      await page.locator('.list').evaluate((e) => (e.scrollTop = e.scrollHeight));
      // Reaching the end loads the next page without anyone pressing a button.
      await page.waitForFunction(() => document.querySelectorAll('.item').length > 50);
      await page.mouse.move(400, 400);
      await page.mouse.wheel(0, 4000);
      await page.waitForTimeout(200);
      h = await pageHeights();
      assert.strictEqual(h.y, 0, 'the page scrolled');
      assert.ok(await page.locator('.list').evaluate((e) => e.scrollTop > 1000), 'the list did not scroll');
      assert.strictEqual(await page.getByRole('navigation', { name: 'Feeds' }).evaluate((e) => Math.round(e.getBoundingClientRect().top)), 0, 'the panes moved');
      await page.getByRole('button', { name: 'Unfollow' }).click();
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
    });

    await step('refreshing one feed shows when it was updated', async () => {
      await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: /^Field Notes/ }).click();
      await page.getByRole('button', { name: 'Refresh', exact: true }).click();
      await page.getByText('Updated just now').waitFor();
      await page.getByText('Feed refreshed').waitFor();
    });

    await step('a feed that starts failing says so, and why', async () => {
      await addFeed(page, site.url + '/flaky.xml');
      await page.getByRole('heading', { name: 'Flaky Feed', level: 1 }).waitFor();
      await page.getByRole('button', { name: 'Refresh', exact: true }).click();
      const alert = page.getByRole('alert');
      await alert.waitFor();
      assert.match(await alert.innerText(), /500 Internal Server Error/);
      // The failure stays on the feed after the message is gone.
      await page.getByText(/Could not be fetched: .*500/).waitFor();
      assert.ok(await page.getByRole('navigation', { name: 'Feeds' }).getByText('Could not be fetched.').count() >= 1);
      await page.getByRole('button', { name: 'Unfollow' }).click();
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
    });

    await step('a summary-only feed says so, and the full article loads on request', async () => {
      await addFeed(page, site.url + '/summaryonly.xml');
      await page.getByRole('heading', { name: 'Summary Only Feed', level: 1 }).waitFor();
      await page.getByRole('button', { name: /Summary story 1\b/ }).click();
      await page.getByRole('heading', { name: 'Summary story 1', level: 2 }).waitFor();
      await page.getByText('This is only the summary the feed provides.').waitFor();
      assert.ok(!(await page.locator('.content').innerText()).includes('The full text of story 1'), 'the full text was already showing');

      await page.getByRole('button', { name: 'Load full article' }).click();
      await page.getByText(/Showing the full article from 127\.0\.0\.1/).waitFor();
      const text = await page.locator('.content').innerText();
      assert.match(text, /The full text of story 1 begins here/);
      assert.ok(!text.trimStart().startsWith('Summary story 1'), 'the headline is repeated at the top of the article');
      assert.match(text, /third paragraph finishes the full text of story 1/);
      assert.ok(!/newsletter|Copyright/.test(text), 'the page around the article came along');
      assert.strictEqual(await page.getByText('This is only the summary the feed provides.').count(), 0, 'the hint stayed after loading');

      // It is kept: after a reload the article opens with its full text, without asking again.
      await page.reload();
      await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: /^Summary Only Feed/ }).click();
      await page.getByRole('button', { name: /Summary story 1\b/ }).click();
      await page.getByText(/Showing the full article from/).waitFor();
    });

    await step('the f key loads the full article, and a feed can do it by itself', async () => {
      await page.keyboard.press('j');
      await page.getByRole('heading', { name: 'Summary story 2', level: 2 }).waitFor();
      await page.keyboard.press('f');
      await page.getByText(/Showing the full article from/).waitFor();
      assert.match(await page.locator('.content').innerText(), /The full text of story 2 begins here/);

      await page.getByLabel('Load full articles automatically').check();
      await page.getByText(/Full articles will load by themselves in Summary Only Feed/).waitFor();
      await page.keyboard.press('j');
      await page.getByRole('heading', { name: 'Summary story 3', level: 2 }).waitFor();
      // Nobody pressed anything: it loads because the feed is set to.
      await page.getByText(/Showing the full article from/).waitFor();
      assert.match(await page.locator('.content').innerText(), /The full text of story 3 begins here/);
    });

    await step('when the full article cannot be loaded the summary stays, and it says why', async () => {
      await page.getByRole('button', { name: /Broken link story/ }).click();
      await page.getByRole('heading', { name: 'Broken link story', level: 2 }).waitFor();
      const alert = page.getByRole('alert');
      await alert.waitFor();
      assert.match(await alert.innerText(), /Could not load the full article\. The site answered "404 Not Found"/);
      assert.match(await page.locator('.content').innerText(), /A teaser whose page is gone/);
      await page.getByText('This is only the summary the feed provides.').waitFor();
      await page.getByLabel('Load full articles automatically').uncheck();
      await page.getByRole('button', { name: 'Unfollow' }).click();
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
    });

    await step('feeds can be filed under a topic and the list grouped by it', async () => {
      const nav = page.getByRole('navigation', { name: 'Feeds' });
      assert.strictEqual(await nav.getByRole('button', { name: 'Group by topic' }).count(), 0, 'no toggle while no feed has a topic');
      await nav.getByRole('button', { name: /^Field Notes/ }).click();
      await page.getByRole('heading', { name: 'Field Notes', level: 1 }).waitFor();
      await page.getByLabel('Topic').fill('Reading');
      await page.getByLabel('Topic').press('Enter');
      await page.getByText('Field Notes is now under Reading').waitFor();
      await nav.getByText('Reading', { exact: true }).waitFor();
      await nav.getByText('Other', { exact: true }).waitFor();
      // Turning grouping off brings the single list back, and the choice sticks.
      await nav.getByRole('button', { name: 'Group by topic' }).click();
      assert.strictEqual(await nav.getByText('Reading', { exact: true }).count(), 0);
      await page.reload();
      await nav.getByRole('button', { name: /^Field Notes/ }).waitFor();
      assert.strictEqual(await nav.getByText('Reading', { exact: true }).count(), 0);
      await nav.getByRole('button', { name: 'Group by topic' }).click();
      await nav.getByText('Reading', { exact: true }).waitFor();
    });

    await step('an administrator can edit the starter list', async () => {
      await page.goto(BASE + '/settings');
      const card = page.getByRole('region', { name: 'Starter feeds' });
      await card.waitFor();
      await card.getByLabel('Topic').fill('Music');
      await card.getByLabel('Feed address').fill(site.url + '/fieldnotes.xml');
      await card.getByRole('button', { name: 'Add to list' }).click();
      await card.getByText('Added.').waitFor();
      await card.getByText('Music', { exact: false }).first().waitFor();
      await card.getByRole('button', { name: 'Remove Field Notes' }).waitFor().catch(() => {});
      await page.goto(BASE + '/');
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
    });

    await step('unfollowing a feed removes it and its articles', async () => {
      await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: /^Hostile Feed/ }).click();
      await page.getByRole('button', { name: 'Unfollow' }).click();
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
      assert.strictEqual(await page.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: /^Hostile Feed/ }).count(), 0);
    });

    await step('importing an OPML file adds its feeds once', async () => {
      await page.goto(BASE + '/settings');
      await page.getByRole('link', { name: /Back to reading/ }).waitFor();
      const opml = await site.opml();
      await page.getByRole('region', { name: 'Subscriptions' }).locator('input[type=file]').setInputFiles({ name: 'subs.opml', mimeType: 'text/xml', buffer: Buffer.from(opml) });
      await page.getByText(/already followed 2/).waitFor();
    });

    await step('exporting gives an OPML file listing the feeds', async () => {
      const body = await page.evaluate(() => fetch('/api/opml').then((r) => r.text()));
      assert.ok(body.includes('/fieldnotes.xml') && body.includes('/systems.atom'), body);
    });

    await step('the dark theme sticks across a reload', async () => {
      await page.getByLabel('Dark').check();
      await page.reload();
      assert.strictEqual(await page.evaluate(() => document.documentElement.getAttribute('data-theme')), 'dark');
      await page.getByLabel('Device').check();
    });

    await step('keyboard shortcuts can be switched off', async () => {
      await page.goto(BASE + '/settings');
      await page.getByLabel('Use keyboard shortcuts').uncheck();
      await page.getByRole('link', { name: /Back to reading/ }).click();
      await page.waitForFunction(() => document.querySelectorAll('.item').length > 0);
      await page.keyboard.press('j');
      await page.waitForTimeout(300);
      assert.strictEqual(await page.locator('.headline').count(), 0, 'j opened an article with shortcuts off');
      await page.getByRole('link', { name: 'Settings' }).click();
      await page.getByLabel('Use keyboard shortcuts').check();
      await page.getByRole('link', { name: /Back to reading/ }).click();
      await page.waitForFunction(() => document.querySelectorAll('.item').length > 0);
      await page.keyboard.press('j');
      await page.locator('.headline').waitFor();
    });

    await step('signing out and back in', async () => {
      await page.goto(BASE + '/');
      await page.getByRole('button', { name: 'Sign out' }).click();
      await page.waitForURL('**/login');
      await page.getByLabel('Username').fill('demo');
      await page.getByLabel('Password', { exact: true }).fill('wrong-password');
      await page.getByRole('button', { name: 'Sign in' }).click();
      await page.getByRole('alert').waitFor();
      await page.getByLabel('Password', { exact: true }).fill('demo-password-1');
      await page.getByRole('button', { name: 'Sign in' }).click();
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
      await page.reload();
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
    });

    await step('an added user starts with a temporary password and sees none of the administrator\'s feeds', async () => {
      await page.goto(BASE + '/settings');
      await page.getByLabel('New username').fill('sam');
      await page.getByRole('button', { name: 'Add user' }).click();
      const temp = await page.getByLabel('Temporary password').inputValue();
      assert.match(temp, /^[a-z0-9]{4}(-[a-z0-9]{4}){3}$/);
      await page.getByRole('button', { name: 'Done' }).click();

      const other = await browser.newContext({ viewport: { width: 1280, height: 800 } });
      const p2 = await other.newPage();
      await p2.goto(BASE + '/');
      await p2.getByLabel('Username').fill('sam');
      await p2.getByLabel('Password', { exact: true }).fill(temp);
      await p2.getByRole('button', { name: 'Sign in' }).click();
      await p2.waitForURL('**/change-password');
      await p2.getByLabel('Temporary password').fill(temp);
      await p2.getByLabel('New password', { exact: true }).fill('sam-password-1');
      await p2.getByLabel('Confirm new password').fill('sam-password-1');
      await p2.getByRole('button', { name: /Change password|Save/ }).click();
      await p2.getByRole('heading', { name: 'Follow your first feed' }).waitFor();
      await other.close();
    });

    await step("signing out clears the reader, so the next person never sees the last one's feeds", async () => {
      await page.goto(BASE + '/');
      await page.waitForFunction(() => document.querySelectorAll('.item').length > 0);
      await page.waitForFunction(() => /^\(\d+\) Huginn$/.test(document.title)); // the tab title carries the unread count
      await page.getByRole('button', { name: 'Sign out' }).click();
      await page.waitForURL('**/login');
      assert.strictEqual(await page.title(), 'Huginn');
      // From here on, watch the page for any trace of the previous user's feeds.
      await page.evaluate(() => {
        window.__leak = false;
        new MutationObserver(() => {
          if (/Systems Weekly|Field Notes/.test(document.body.innerText)) window.__leak = true;
        }).observe(document.body, { subtree: true, childList: true, characterData: true });
      });
      await page.getByLabel('Username').fill('sam');
      await page.getByLabel('Password', { exact: true }).fill('sam-password-1');
      await page.getByRole('button', { name: 'Sign in' }).click();
      await page.getByRole('heading', { name: 'Follow your first feed' }).waitFor();
      // Sam has not been offered the starter feeds yet, so the picker is open; turn it down.
      await page.getByRole('dialog', { name: 'Start with a few feeds' }).getByRole('button', { name: 'Start empty' }).click();
      await page.waitForTimeout(500);
      assert.strictEqual(await page.evaluate(() => window.__leak), false, "the previous user's feeds showed for the next user");
      await page.getByRole('button', { name: 'Sign out' }).click();
      await page.waitForURL('**/login');
      await page.getByLabel('Username').fill('demo');
      await page.getByLabel('Password', { exact: true }).fill('demo-password-1');
      await page.getByRole('button', { name: 'Sign in' }).click();
      await page.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
    });

    await step('on a phone one pane shows at a time', async () => {
      const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, storageState: await ctx.storageState() });
      const p = await phone.newPage();
      await p.goto(BASE + '/');
      await p.getByRole('heading', { name: 'All items', level: 1 }).waitFor();
      assert.strictEqual(await p.getByRole('navigation', { name: 'Feeds' }).isVisible(), false);
      await p.locator('.item').first().click();
      await p.getByRole('button', { name: 'Back to articles' }).waitFor();
      assert.strictEqual(await p.getByRole('region', { name: 'Articles' }).isVisible(), false);
      // Focus moves into the article instead of staying on a button that is no longer on screen.
      await p.waitForFunction(() => document.activeElement?.classList.contains('headline'));
      // The browser's own Back button returns to the list, and focus returns to the article that was open.
      await p.goBack();
      await p.getByRole('region', { name: 'Articles' }).waitFor();
      assert.strictEqual(await p.getByRole('region', { name: 'Article', exact: true }).isVisible(), false);
      await p.waitForFunction(() => document.activeElement?.classList.contains('item'));
      assert.ok(p.url().startsWith(BASE), 'Back left the app');
      // The on-screen back button agrees with it.
      await p.locator('.item').first().click();
      await p.getByRole('button', { name: 'Back to articles' }).click();
      await p.getByRole('region', { name: 'Articles' }).waitFor();
      await p.getByRole('button', { name: 'Show feeds' }).click();
      await p.getByRole('navigation', { name: 'Feeds' }).waitFor();
      await p.getByRole('navigation', { name: 'Feeds' }).getByRole('button', { name: /^Systems Weekly/ }).click();
      await p.getByRole('heading', { name: 'Systems Weekly', level: 1 }).waitFor();
      const overflow = await p.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
      assert.strictEqual(overflow, false, 'the page scrolls sideways on a phone');
      await phone.close();
    });

    assert.deepStrictEqual(errors, [], 'the page threw errors');
    console.log(`\n${passed} checks passed`);
  } catch (e) {
    console.log('\nStopped at the first failure.');
    process.exitCode = 1;
  } finally {
    await browser.close();
    site.close();
  }
})();
