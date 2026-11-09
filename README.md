# gofs

A Go reimplementation of [dufs](https://github.com/sigoden/dufs), a distinctive
utility file server. It supports static serving, uploading, searching, access
control and WebDAV.

Runtime dependencies: only
[`github.com/zxysilent/logs`](https://github.com/zxysilent/logs) (logging).
Everything else uses the Go standard library.

The web UI is a modern single-page application (left directory tree + right file
browser with previews), implemented as a build-free native ES module in
`web/dist/` and embedded into the Go binary via `go:embed`. No npm/Vite build
step is required.

## Build

```sh
go build -o gofs .
```

The frontend lives directly in `web/dist/` (`index.html`, `app.js`,
`style.css`) and is embedded at compile time — no additional tooling needed.

## Usage

```
gofs [OPTIONS] [serve-path]

Options:
  -c, --config <file>        Specify configuration file
  -b, --bind <addrs>         Specify bind address or unix socket
  -p, --port <port>          Specify port to listen on [default: 5000]
      --path-prefix <path>   Specify a path prefix
      --hidden <value>       Hide paths from directory listings
  -a, --auth <rules>         Add auth roles, e.g. user:pass@/dir1:rw,/dir2
  -A, --allow-all            Allow all operations
      --allow-upload         Allow upload files/folders
      --allow-delete         Allow delete files/folders
      --allow-search         Allow search files/folders
      --allow-symlink        Allow symlink to files/folders outside root
      --allow-archive        Allow download folders as archive file
      --allow-hash           Allow ?hash query to get file sha256 hash
      --enable-cors          Enable CORS
      --render-index         Serve index.html for directories (404 if absent)
      --render-try-index     Serve index.html, fall back to listing
      --render-spa           Serve a Single Page Application
      --assets <path>        Override the built-in web assets
      --log-format <format>  Customize http log format
      --log-file <file>      Save logs to a file
      --compress <level>     Zip level: none|low|medium|high [default: low]
      --tls-cert <path>      TLS certificate for HTTPS
      --tls-key <path>       TLS private key
  -h, --help                 Print help
  -V, --version              Print version
```

All options can also be set via `DUFS_*` environment variables (for example
`DUFS_PORT=5000`, `DUFS_AUTH=admin:123@/:rw`). A YAML configuration file is
supported through `--config`.

## Features

- Static file serving with HTTP range requests (`Range`, `If-Range`, ETag,
  `Last-Modified`, conditional requests).
- Directory listing as HTML, JSON (`?json`), plain names (`?simple`) or a
  text-client friendly page (`?noscript`).
- Upload (PUT), resumable upload (`PATCH` + `X-Update-Range`), delete, create
  directory (`MKCOL`), copy and move (WebDAV).
- Download a folder as a `.zip` archive (`?zip`).
- File search (`?q=`).
- File editor/viewer (`?edit`, `?view`) and sha256 hash (`?hash`).
- Account based access control with Basic and Digest authentication, including
  sha-512 hashed passwords (`$6$...`).
- WebDAV (`PROPFIND`, `PROPPATCH`, `LOCK`, `UNLOCK`, `COPY`, `MOVE`, `MKCOL`).
- HTTPS via `--tls-cert`/`--tls-key`, unix socket via `--bind /path.sock`.
- Health check at `__dufs__/health`.

## Examples

```sh
# serve current directory read-only
gofs

# allow all operations
gofs -A

# require username/password
gofs -a 'admin:123@/:rw'

# serve a single page application
gofs --render-spa

# https
gofs --tls-cert cert.pem --tls-key key.pem
```

## Layout

| File | Purpose |
|------|---------|
| `main.go` | entry point, listener setup, signal handling |
| `args.go` | CLI/env/config parsing |
| `server.go` | HTTP + WebDAV request handling |
| `auth.go` | access control, Basic/Digest auth, sha512-crypt |
| `logger.go` | customizable access log format |
| `utils.go` | URI encoding, glob, range parsing, content type |
| `noscript.go` | text-client directory listing |
| `assets/` | embedded web UI (HTML/CSS/JS/favicon) |
