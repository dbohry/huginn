// A small website of test feeds, served on localhost so the checks never touch the internet.
const http = require('http');

const rss = (title, link, items) => `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>${title}</title><link>${link}</link><description>${title}</description>
    ${items.map((i) => `<item>
      <title>${i.title}</title><link>${i.link}</link><guid isPermaLink="false">${i.id}</guid>
      <pubDate>${i.date}</pubDate>
      <description><![CDATA[${i.summary}]]></description>
      <content:encoded><![CDATA[${i.body}]]></content:encoded>
    </item>`).join('')}
  </channel>
</rss>`;

const atom = (title, link, items) => `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>${title}</title><link href="${link}" rel="alternate"/><id>urn:feed:${title}</id>
  <updated>2026-11-01T12:00:00Z</updated>
  ${items.map((i) => `<entry>
    <title>${i.title}</title><link href="${i.link}"/><id>${i.id}</id><updated>${i.iso}</updated>
    <summary>${i.summary}</summary>
    <content type="html"><![CDATA[${i.body}]]></content>
  </entry>`).join('')}
</feed>`;

const fieldNotes = [
  { id: 'fn1', title: 'A week with only a paper calendar', link: 'https://example.com/fn1', date: 'Mon, 02 Nov 2026 09:00:00 +0000',
    summary: 'What changed when every reminder had to be written by hand.',
    body: '<p>On Monday I turned off every notification and bought a calendar with a page per week.</p><h3>What I noticed</h3><ul><li>Writing takes fifteen seconds</li><li>That friction is a filter</li></ul><blockquote>Things not worth fifteen seconds did not make the page.</blockquote>' },
  { id: 'fn2', title: 'Walking the long way to the station', link: 'https://example.com/fn2', date: 'Sun, 01 Nov 2026 18:00:00 +0000',
    summary: 'An extra eleven minutes, and the best part of the day.',
    body: '<p>The long route passes a bakery, a school and a stretch of canal.</p><p>I leave the headphones at home on this walk.</p>' },
  { id: 'fn3', title: 'Notes on keeping a notebook', link: 'https://example.com/fn3', date: 'Sat, 31 Oct 2026 08:00:00 +0000',
    summary: 'One notebook, one pen, no system.',
    body: '<p>A notebook only needs to be <em>there</em>.</p><pre>date, thing, done?</pre>' },
];
const systems = [
  { id: 'sw1', title: 'Small servers, long uptimes', link: 'https://example.org/sw1', iso: '2026-11-02T08:00:00Z',
    summary: 'Why one process and one file outlasts most architectures.',
    body: '<p>Most software that has run for ten years on one machine was boring on purpose.</p>' },
  { id: 'sw2', title: 'Plain text as an interface', link: 'https://example.org/sw2', iso: '2026-10-30T08:00:00Z',
    summary: 'Files you can read, grep and diff.',
    body: '<p>Plain text survives because every tool can read it.</p>' },
];
const gardens = [
  { id: 'tg1', title: 'Growing herbs on a north-facing sill', link: 'https://example.net/tg1', date: 'Mon, 02 Nov 2026 06:30:00 +0000',
    summary: 'Mint, chives and parsley all tolerate less light than the packets suggest.',
    body: '<p>A north-facing window gets no direct sun, but bright indirect light is enough for leafy herbs.</p><p>Rotate the pots a quarter turn each week and water only when the top knuckle of soil is dry.</p>' },
  { id: 'tg2', title: 'Compost, without the mystery', link: 'https://example.net/tg2', date: 'Fri, 30 Oct 2026 10:00:00 +0000',
    summary: 'Greens, browns and patience: the only three rules that matter.',
    body: '<p>Roughly equal parts wet kitchen waste and dry leaves or cardboard. Turn it when you remember. Wait.</p>' },
  { id: 'tg3', title: 'What to sow in November', link: 'https://example.net/tg3', date: 'Wed, 28 Oct 2026 10:00:00 +0000',
    summary: 'Broad beans, garlic and a last row of lettuce under a cloche.',
    body: '<p>Garlic wants a cold spell, so get it in the ground now.</p>' },
];
// A long feed, for checking that a long list scrolls inside its pane and not the page.
const many = Array.from({ length: 120 }, (_, i) => ({
  id: 'm' + i, title: `Long list article ${i}: a headline long enough to wrap onto a second line`, link: 'https://example.com/m' + i,
  date: new Date(Date.UTC(2026, 10, 2, 12, 0, 0) - i * 60000).toUTCString(),
  summary: `Summary of article ${i}, long enough that the two line clamp has something to cut off.`, body: `<p>Body ${i}</p>`,
}));
// A feed written by someone hostile. None of this may run or show.
const hostile = [
  { id: 'h1', title: 'Totally harmless post', link: 'javascript:window.__pwned=1', date: 'Mon, 02 Nov 2026 07:00:00 +0000',
    summary: '<script>window.__pwned=1</script>Summary text',
    body: '<p onclick="window.__pwned=1">Visible text</p><script>window.__pwned=1</script>' +
      '<img src="https://127.0.0.1:1/x.png" onerror="window.__pwned=1" alt="broken">' +
      '<iframe src="http://127.0.0.1:1/"></iframe><style>body{display:none}</style>' +
      '<a href="javascript:window.__pwned=1">click me</a><a href="https://example.com/ok">a fine link</a>' },
];

function start(port = 0) {
  let flakyHits = 0;
  const server = http.createServer((req, res) => {
    const send = (type, body) => { res.writeHead(200, { 'Content-Type': type }); res.end(body); };
    const base = `http://127.0.0.1:${server.address().port}`;
    switch (req.url) {
      case '/fieldnotes.xml': return send('application/rss+xml', rss('Field Notes', base + '/fieldnotes/', fieldNotes));
      case '/flaky.xml': // works once, then fails
        if (++flakyHits > 1) { res.writeHead(500); return res.end('down'); }
        return send('application/rss+xml', rss('Flaky Feed', base + '/flaky/', [{ id: 'f1', title: 'Only article', link: 'https://example.com/f1', date: 'Mon, 02 Nov 2026 05:00:00 +0000', summary: 'It worked once.', body: '<p>It worked once.</p>' }]));
      case '/many.xml': return send('application/rss+xml', rss('Long Feed', base + '/many/', many));
      case '/gardens.xml': return send('application/rss+xml', rss('Tiny Gardens', base + '/gardens/', gardens));
      case '/systems.atom': return send('application/atom+xml', atom('Systems Weekly', base + '/systems/', systems));
      case '/systems/': return send('text/html', `<!doctype html><html><head><title>Systems Weekly</title><link rel="alternate" type="application/atom+xml" href="/systems.atom"></head><body>Home</body></html>`);
      case '/hostile.xml': return send('application/rss+xml', rss('Hostile Feed', base + '/hostile/', hostile));
      case '/notafeed.html': return send('text/html', '<html><body>nothing here</body></html>');
      case '/subscriptions.opml': return send('text/x-opml', `<?xml version="1.0"?><opml version="2.0"><head/><body>
        <outline type="rss" text="Field Notes" xmlUrl="${base}/fieldnotes.xml"/>
        <outline type="rss" text="Systems Weekly" xmlUrl="${base}/systems.atom"/></body></opml>`);
      default: res.writeHead(404); res.end('not found');
    }
  });
  return new Promise((resolve) => server.listen(port, '127.0.0.1', () => {
    const url = `http://127.0.0.1:${server.address().port}`;
    resolve({ url, close: () => server.close(), opml: () => fetch(url + '/subscriptions.opml').then((r) => r.text()) });
  }));
}

module.exports = { start };
