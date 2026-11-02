<p align="center">
  <img src="docs/logo.svg" alt="Huginn" width="96" height="96">
</p>

<h1 align="center">Huginn</h1>

<p align="center">
  A small, fast, self-hosted RSS reader.<br>
  One Go binary and one SQLite file.
</p>

> **Status: early development.** The server starts and reports its health, but
> there are no feeds or interface yet. See the [MVP plan](docs/PLAN.md).

## Run it

```sh
docker run -d --name huginn --restart unless-stopped \
  -p 8080:8080 -v huginn-data:/data \
  dbohry/huginn:latest
```

Everything Huginn stores lives in the `/data` volume.

### Settings

Both are optional.

| Variable | Default | Meaning |
|---|---|---|
| `HUGINN_ADDR` | `:8080` | Address the server listens on |
| `HUGINN_DATA_DIR` | `/data` | Directory that holds the database |

## Develop

You need Go 1.26. `make help` lists the targets: `make build`, `make run`,
`make test` and `make lint`.

## Name

Huginn ("thought") is one of Odin's two ravens. Every day it flies out over the
world and returns with the news.

## License

[MIT](LICENSE)
