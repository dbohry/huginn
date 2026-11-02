# Huginn MVP plan

Huginn is a small, fast, self-hosted RSS reader. One Go binary and one SQLite
file, the same way [Bifrost](https://github.com/dbohry/bifrost) is built.

The name comes from Odin's raven Huginn ("thought"), who flies out every day
and brings back the news.

## Goal

Run one process, subscribe to feeds, read them in a three-pane interface, and
keep track of what has been read. Nothing else.

## Stack

Same as Bifrost, so the two projects can share habits and scaffolding.

| Layer | Choice |
|---|---|
| Backend | Go, built as one static binary |
| Database | SQLite (WAL mode, pure-Go `modernc.org/sqlite`), stored in a `/data` volume |
| Feeds | `gofeed` to parse RSS, Atom and JSON Feed |
| HTML safety | `bluemonday` allow-list sanitizer for article content |
| Frontend | Svelte 5 + Vite + TypeScript single-page app, embedded in the binary |
| Styling | Plain CSS with theme variables, no component library |
| Distribution | Docker, multi-architecture (`amd64` and `arm64`), published to Docker Hub |
| Checks | `go test -race`, `go vet`, `svelte-check`, a 100 KB gzipped bundle budget, Playwright + axe in `e2e/` |

## Scope

### In the MVP

- Accounts like Bifrost: the first visitor creates the administrator, an
  administrator adds other users, argon2id passwords, sessions in SQLite, login
  rate limiting and a `reset-password` subcommand. Every user has their own
  feeds and read state.
- Add and remove a feed by URL, including discovery from a site's home page.
- OPML import and export.
- Background refresh every 30 minutes and a manual refresh. Conditional requests
  with `ETag` and `Last-Modified`, timeouts, a size limit and a polite
  `User-Agent`.
- RSS 2.0, Atom and JSON Feed. Items are de-duplicated by `guid`, falling back to
  the link.
- The three-pane interface from the mockup: feeds with unread counts, an article
  list and a reading pane. It also works at phone width.
- Mark read and unread, and mark all read.
- Sanitized article content and an "Open original" link.
- Light and dark theme, following the device.

### Not in the MVP

Full-text extraction, search, folders and tags, starring, sync APIs for mobile
apps, keyboard shortcuts and custom themes. Several of these are small and come
straight after the MVP.

## Data model (first draft)

- `meta(key, value)`
- `feeds(id, user_id, url, title, site_url, etag, last_modified, last_fetched_at, last_error)`
- `items(id, feed_id, guid, title, url, author, published_at, summary, content_html, is_read)`,
  unique on `(feed_id, guid)`
- `users` and `sessions`, as in Bifrost

Feeds belong to one user, like Bifrost's apps. If two users follow the same feed
it is fetched twice. That keeps read state trivial, and it is cheap at this size.

## Milestones

All seven are done.

| # | Milestone | Done when |
|---|---|---|
| 1 | **Skeleton.** Go module, config, SQLite with migrations, `feeds` and `items` tables, `/healthz`, `healthcheck` subcommand, Makefile, Dockerfile, CI, release workflow | CI is green and the container reports healthy |
| 2 | **Fetch and parse.** Fetcher with timeouts, size limit, conditional requests, de-duplication, per-feed error tracking, background refresher | Tests pass against fixture feeds (RSS, Atom, JSON Feed, malformed) |
| 3 | **Accounts.** First-run setup, login, sessions, users, rate limiting, `reset-password`, feeds scoped to a user | Setup and login work, and a user cannot see another user's feeds |
| 4 | **API.** Feeds, items, mark read, refresh, OPML import and export | Every endpoint has a test |
| 5 | **Interface.** Svelte app with the three panes, logo, themes, phone layout, bundle budget in CI | You can subscribe to a feed and read it in a browser |
| 6 | **Hardening.** HTML sanitizer, feed discovery, backoff for failing feeds, per-host politeness, browser and accessibility checks | A feed containing `<script>` cannot run code, and the accessibility scan is clean |
| 7 | **Ship.** README with screenshots, `docs/DEVELOPMENT.md`, Docker Hub image, favicon | A fresh machine runs it with one `docker run` |

Deploying automatically (the Portainer webhook Bifrost uses) is left until
after the MVP.

## How the work differed from the plan

- The HTML sanitizer was built with the API (milestone 4), because that is where article content first reaches a browser. Milestone 6 added the hostile-feed tests.
- Backoff for failing feeds and one-fetch-per-host politeness were built with the fetcher (milestone 2).
- Unfollowing a feed from the interface, showing a feed's last error, and shifting headings inside articles below the article title were added when the browser and accessibility checks showed they were needed.
- The browser checks (`make e2e`) are not part of CI yet, as in Bifrost.
- The Docker image could not be built while this was written (there was no Docker daemon in the environment), so CI's `image` job is its first real build.

## Main risks

- **Messy real-world feeds.** Bad XML, odd encodings, missing dates. Collect real
  feeds as test fixtures in milestone 2.
- **Unsafe HTML.** Always sanitize with an allow-list, never with hand-written
  rules, and set a strict Content-Security-Policy.
- **Bundle size.** The 100 KB budget is checked in CI from milestone 5.
- **Scope creep.** Search, folders and sync wait until the MVP ships.
