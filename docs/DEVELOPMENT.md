# Development

You need Go 1.26 and Node 22. `make help` lists the targets:

| Command | What it does |
|---|---|
| `make dev` | Go server on :8080 and the Vite dev server on :5173, with hot reload |
| `make build` | Builds the frontend, then `bin/huginn` with it embedded |
| `make test` | Go tests with the race detector |
| `make lint` | `gofmt`, `go vet` and `svelte-check` |
| `make size` | Checks the built frontend against the 100 KB gzipped budget |
| `make e2e` | Browser checks of the main journeys and an accessibility scan of every screen, using `bin/huginn`, so run `make build` first |
| `make image` | Builds the Docker image |

Browser checks live in `e2e/` and need Chromium: run `npm install` in that directory once, then `make e2e`. Each check starts its own server on an empty data directory and serves its own test feeds from localhost, so nothing touches the internet. To run one by hand, start Huginn on an empty directory and use `BASE=http://127.0.0.1:8080 npm run flows` or `npm run a11y`. The accessibility scan (axe-core) runs on a phone and a desktop, in light and dark, and must report nothing.

`npm run screenshots` in `e2e/` rewrites the README screenshots in `docs/screenshots` from the same test feeds.

## Layout

- `cmd/huginn`: the binary and its subcommands (`serve`, `reset-password`, `healthcheck`, `version`)
- `internal/config`, `internal/db`, `internal/store`, `internal/auth`, `internal/server`: configuration, SQLite with migrations, data access, passwords and rate limiting, HTTP handlers
- `internal/feed`: fetching (conditional requests, size limit, timeout), parsing RSS, Atom and JSON Feed, and finding the feed a web page advertises
- `internal/refresh`: the background refresher: due feeds once a minute, four at a time, one at a time per host, exponential backoff after failures
- `internal/sanitize`: the allow-list that cleans article HTML, and the plain-text snippets. A test feeds it hostile input
- `internal/opml`: OPML import and export
- `internal/webui`: embeds the built frontend
- `frontend`: the Svelte app. `npm run build` writes into `internal/webui/dist`
- Database migrations are numbered SQL files in `internal/db/migrations`, applied in order at start-up

## CI and releases

CI (`.github/workflows/ci.yml`) runs on every pull request: `gofmt`, `go vet` and the Go tests, the frontend type check, build and size budget, and a build of the Docker image that must report healthy. It can also be started by hand from the Actions tab. The browser checks (`make e2e`) are not in CI yet, so run them before opening a pull request that changes the interface.

The Release workflow (`.github/workflows/release.yml`) builds the image for amd64 and arm64 and publishes `dbohry/huginn:latest` (and `dbohry/huginn:main-<commit>`, to pin an exact build) to Docker Hub on every merge to main, except changes to docs only. It needs two repository secrets: `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` (an access token with read and write rights). There is no automatic deploy and there are no version tags for now.
