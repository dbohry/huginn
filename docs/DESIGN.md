# Design notes

Why Huginn is built the way it is.

## Goals

- Use few resources, at idle and under load.
- Read comfortably on a phone, a tablet and a desktop.
- Be safe to point at feeds written by strangers.
- Stay simple: one binary, one database file, no other services.

## Architecture

| Layer | Choice |
|---|---|
| Backend | Go, built as one static binary |
| Database | SQLite (WAL mode, pure-Go driver), stored in a `/data` volume |
| Feeds | `gofeed` to parse, `bluemonday` to sanitize |
| Frontend | Svelte 5 + Vite + TypeScript, a single-page app embedded in the binary |
| Styling | Plain CSS with a few colour tokens, no component library |
| Distribution | Docker, `amd64` and `arm64` |

It is the same stack as [Bifrost](https://github.com/dbohry/bifrost), and the accounts, sessions and password code started there.

### Fetching

- A feed is fetched with `If-None-Match` and `If-Modified-Since`, so an unchanged feed answers `304` and costs nothing to parse.
- The refresher wakes once a minute, takes the feeds that are due, and fetches four at a time but never two from the same host. Each feed is due every 30 minutes.
- After a failure the wait doubles each time, up to 12 hours, and success resets it. The last error is kept on the feed and shown in the interface.
- A response over 5 MB, or one that takes over 20 seconds, is a failure.
- With `HUGINN_BLOCK_PRIVATE_NETWORKS=true` the fetcher refuses private, loopback, link-local and carrier-grade-NAT addresses. The check runs on the address actually dialled, after the name is resolved, so redirects and rebinding names are covered.
- An article is identified by the feed's own id, else its link, else a hash of its title and date. Fetching again updates an edited article but never changes whether it was read. An article with no date keeps the time it was first seen.
- Each feed keeps its newest 1000 articles.

### Safety

- Everything from a feed that reaches the page goes through `internal/sanitize`. Article HTML is cleaned with an allow-list (no scripts, frames, forms, styles or `javascript:`/`data:` addresses), links open in a new tab with `noopener noreferrer`, and headings are shifted below the article's own title. Snippets and titles are plain text.
- The interface inserts the cleaned HTML only because the server has already cleaned it. A browser check loads a hostile feed and asserts that nothing ran.
- The Content-Security-Policy allows scripts and styles from the app itself, and images over HTTPS.
- Passwords are hashed with argon2id, sessions are random tokens stored hashed, sign-in is rate limited, and changing a request's state from another site is refused.

### Accounts

Like Bifrost: the first account is the administrator and chooses their password; the administrator adds others with a temporary password that must be changed at first sign-in. Feeds belong to one user. If two users follow the same feed it is fetched twice, which keeps read state trivial and is cheap at this size.

### Interface

Three panes on a wide screen (feeds, articles, the reader). Below 900 px only one pane shows at a time, and the page keeps one `main` landmark and a level-one heading in every state. The bundle has a 100 KB gzipped budget, checked in CI.

## Not in the first version

Search, folders and tags, starring, keyboard shortcuts, full-text extraction for feeds that only publish a summary, a sync API for mobile apps, and image proxying (which would stop sites seeing who reads their pictures).
