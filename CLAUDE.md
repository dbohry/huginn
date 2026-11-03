# Working in this repository

## Run the e2e tests before opening or updating a pull request

CI runs the browser tests in `e2e/` (`make e2e`) on every pull request. Never open a pull
request, or push to one, without having run them yourself and seen them pass. If your change
alters behavior a test describes, update the test in the same commit.

```bash
make build
(cd e2e && npm ci)
make e2e
```

If the Playwright version in `e2e/` wants a Chromium build that is not installed, point
`PLAYWRIGHT_BROWSERS_PATH` at a directory that links the installed one under the expected name.
Do not change `e2e/package.json` for that.
