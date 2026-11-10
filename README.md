# gofs

[![CI](https://github.com/zxysilent/gofs/actions/workflows/ci.yml/badge.svg)](https://github.com/zxysilent/gofs/actions/workflows/ci.yml)
[![Release](https://github.com/zxysilent/gofs/actions/workflows/release.yml/badge.svg)](https://github.com/zxysilent/gofs/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/zxysilent/gofs)](https://github.com/zxysilent/gofs/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**English** | [简体中文](README.zh-CN.md)

A distinctive utility file server — a Go reimplementation of [dufs](https://github.com/sigoden/dufs)
with a modern Vue 3 web UI.

Serves static files, uploads, searches, access control and WebDAV. The whole
thing — Go backend **and** the SPA frontend — compiles into a **single binary**
with no runtime dependencies beyond the Go standard library plus one tiny
logging package.

![gofs web UI — light theme](docs/screenshots/01-list-light-zh.png)

## Features

**Web UI** (Vue 3 + TypeScript, embedded in the binary)

- Split view: lazy-loading directory tree on the left, file list/grid on the right
- Breadcrumb navigation with real-path routing — deep links and reloads keep
  their position, entirely client-side
- **Light / dark / auto theme** and **Chinese / English UI**, both persisted
  in the browser
- In-browser preview: images, video, audio, PDF, Markdown, code, JSON, text
- Markdown rendering with GFM tables, **math via KaTeX** (`$...$`, `$$...$$`,
  `\(...\)`, `\[...\]`), **Mermaid diagrams** (```` ```mermaid ```` fences),
  images and links resolved relative to the document. Both renderers are
  lazy-loaded only when the document actually needs them, and diagrams are
  re-themed when you switch light/dark
- In-place text editing with `Ctrl/Cmd+S` to save and a Markdown preview toggle
- Live upload progress, zip download of any folder, recursive search
- Drag the divider to resize the sidebar; long names truncate with an ellipsis

**Protocol / API**

- Static serving with HTTP range requests (`Range`, `If-Range`, ETag,
  `Last-Modified`, conditional requests)
- Directory listings as HTML, JSON (`?json`), plain names (`?simple`) or a
  text-client-friendly page (`?noscript`)
- Upload (`PUT`), resumable upload (`PATCH` + `X-Update-Range`), delete,
  `MKCOL`, `COPY`, `MOVE` and the rest of WebDAV
- Folder download as `.zip` (`?zip`), recursive search (`?q=`), SHA256
  (`?hash`), online editor (`?edit` / `?view`)
- Account-based access control: Basic and Digest auth, sha512-crypt passwords
  (`$6$...`), per-path read/write rules and tokens
- HTTPS via `--tls-cert` / `--tls-key`, unix socket via `--bind /path.sock`
- Health check at `/gofs-healthz`
- **Two built-in virtual Markdown documents** (see below), generated per
  request and available in Chinese and English

## Screenshots

All screenshots below are real captures of the running application.

| Light | Dark |
|-------|------|
| ![list light](docs/screenshots/01-list-light-zh.png) | ![list dark](docs/screenshots/02-list-dark-zh.png) |

Grid view (dark theme):

![grid view](docs/screenshots/03-grid-dark-zh.png)

Markdown preview and in-place editor:

| Preview | Edit |
|---------|------|
| ![preview](docs/screenshots/04-preview-markdown.png) | ![editor](docs/screenshots/05-editor.png) |

Built-in guide and agent manual (tabs in the help dialog):

| Guide | Agent manual |
|-------|--------------|
| ![help](docs/screenshots/06-help-dialog.png) | ![agent](docs/screenshots/07-agent-manual.png) |

English UI:

| Light | Dark |
|-------|------|
| ![en light](docs/screenshots/08-list-light-en.png) | ![en dark](docs/screenshots/09-list-dark-en.png) |

## Install

Prebuilt binaries are published for every tagged release — no Go toolchain
required.

### One-line install (Linux / macOS / FreeBSD)

```sh
curl -fsSL https://raw.githubusercontent.com/zxysilent/gofs/master/install.sh | bash
```

If you prefer not to pipe into a shell, the same script works from a subshell:

```sh
bash -c "$(curl -fsSL https://raw.githubusercontent.com/zxysilent/gofs/master/install.sh)"
```

The script detects your OS/architecture, downloads the matching archive,
verifies it against the published `SHA256SUMS`, and installs `gofs` into
`/usr/local/bin`.

Pin a version, or choose where it lands:

```sh
# specific version
curl -fsSL https://raw.githubusercontent.com/zxysilent/gofs/master/install.sh | bash -s -- --version 0.1.2

# no sudo needed
curl -fsSL https://raw.githubusercontent.com/zxysilent/gofs/master/install.sh | bash -s -- --dir "$HOME/.local/bin"

# environment variables work too
GOFS_INSTALL_DIR="$HOME/.local/bin" bash -c "$(curl -fsSL https://raw.githubusercontent.com/zxysilent/gofs/master/install.sh)"
```

Run `install.sh --help` for all options (`--dry-run`, `GOFS_BASE_URL` for
mirrors, `GOFS_VERIFY=0` to skip the checksum).

### Manual download

Grab the archive for your platform from the
[releases page](https://github.com/zxysilent/gofs/releases), verify it, and put
the binary on your `PATH`:

```sh
ver=0.1.2
base="https://github.com/zxysilent/gofs/releases/download/v${ver}"
curl -fsSLO "${base}/SHA256SUMS"
curl -fsSLO "${base}/gofs_${ver}_linux_amd64.tar.gz"
sha256sum -c <(grep 'linux_amd64' SHA256SUMS)
tar -xzf "gofs_${ver}_linux_amd64.tar.gz"
sudo install -m 0755 gofs /usr/local/bin/gofs
```

| Platform | Architectures |
|----------|---------------|
| Linux | `amd64`, `arm64`, `armv6` (also runs on armv7), `386` |
| macOS | `amd64`, `arm64` (Apple Silicon) |
| Windows | `amd64`, `arm64` (`.zip`) |
| FreeBSD | `amd64` |

Archives are named `gofs_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) and
contain the binary plus `LICENSE` and both READMEs.

### Docker-free systemd example

```ini
[Unit]
Description=gofs file server
After=network-online.target

[Service]
ExecStart=/usr/local/bin/gofs --port 80 --allow-search --allow-archive /srv/data
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## Build from source

Requires Go 1.24+.

```sh
# The built frontend assets are already committed under web/dist,
# so a plain build needs no Node.js at all:
go build -o gofs .
```

The web UI lives in `web/` (Vue 3 + Vite + TypeScript). Compiled assets in
`web/dist/` are committed and embedded via `//go:embed web/dist`, so `go build`
works without npm.

Only when you change the frontend do you need to rebuild it:

```sh
cd web
bash install-deps.sh          # npm install + npm run build
```

A release build bakes the version in at link time:

```sh
go build -trimpath -ldflags "-s -w -X main.version=0.1.2" -o gofs .
```

## Usage

```
gofs [OPTIONS] [serve-path]

Options:
  -c, --config <file>        Specify configuration file
  -b, --bind <addrs>         Specify bind address or unix socket
  -p, --port <port>          Specify port to listen on [default: 5000]
      --path-prefix <path>   Specify a path prefix
      --hidden <value>       Hide paths from directory listings, e.g. tmp,*.log,*.lock
  -a, --auth <rules>         Add auth roles, e.g. user:pass@/dir1:rw,/dir2
  -A, --allow-all            Allow all operations
      --allow-upload         Allow upload files/folders
      --allow-delete         Allow delete files/folders
      --allow-search         Allow search files/folders
      --allow-symlink        Allow symlink to files/folders outside root directory
      --allow-archive        Allow download folders as archive file
      --allow-hash           Allow ?hash query to get file sha256 hash
      --enable-cors          Enable CORS, sets `Access-Control-Allow-Origin: *`
      --render-index         Serve index.html when requesting a directory, returns 404 if not found index.html
      --render-try-index     Serve index.html when requesting a directory, returns directory listing if not found index.html
      --render-spa           Serve SPA (Single Page Application)
      --assets <path>        Set the path to the assets directory for overriding the built-in assets
      --log-file <file>      Specify the file to save logs to, other than stdout/stderr
      --log-level <level>    Set the log level [default: info] [possible values: debug, info, warn, error]
      --compress <level>     Set zip compress level [default: low] [possible values: none, low, medium, high]
      --completions <shell>  Print shell completion script for <shell>
      --tls-cert <path>      Path to an SSL/TLS certificate to serve with HTTPS
      --tls-key <path>       Path to the SSL/TLS certificate's private key
  -h, --help                 Print help
  -V, --version              Print version
```

Options can also be set through `GOFS_*` environment variables (for example
`GOFS_PORT=5000`, `GOFS_AUTH=admin:123@/:rw`) or a YAML config file via
`--config`.

### Examples

```sh
# serve the current directory read-only
gofs

# allow all operations on a specific directory
gofs -A /srv/data

# read-only share with search and zip download (a typical public share)
gofs --allow-search --allow-archive --allow-hash --port 80 /srv/data

# require a username/password with write access
gofs -a 'admin:123@/:rw'

# HTTPS
gofs --tls-cert cert.pem --tls-key key.pem
```

## Access log

Two structured records per request, sharing one `trace` — the request line and
the result line. No custom template to maintain, and never a second copy of the
timestamp inside a quoted message:

```
time=2026-11-10T15:53:31.244 level=INF trace=cfyk5fcc caller=xbegin ip=127.0.0.1 GET=/gamma
time=2026-11-10T15:53:31.244 level=INF trace=cfyk5fcc caller=xclose status=200 size=425 dur=116.841µs
```

The request record carries who asked for what, the result record carries how it
went. Each field appears exactly once:

| Field | Line | Meaning |
|-------|------|---------|
| `trace` | both | request id, also returned as the `X-Request-ID` response header |
| `caller` | both | `xbegin` or `xclose` |
| `ip` | begin | client address |
| `<METHOD>` | begin | method as the field name, path and query as the value (`GET=/a/b.md`) |
| `status` / `size` / `dur` | end | response status, body bytes, handling time |
| `err` | end | error message, only on failure (logged at error level) |

Both lines are logged at info level, so pairing them needs no extra flag:

```sh
grep 'trace=cfyk5fcc' gofs.log
```

An upstream `X-Request-ID` (8–64 alphanumerics) is reused when present, so the
id stays consistent behind a reverse proxy. Static assets are not logged.

## Built-in virtual documents

Two Markdown documents are generated on request, so the URLs inside them always
match the address the client actually used (friendly to reverse proxies and
`--path-prefix` deployments). A real file with the same name on disk takes
precedence.

| Path | Audience | Content |
|------|----------|---------|
| `/gofs-readme.md` | humans | features, UI usage, deployment notes |
| `/gofs-agent.md` | scripts / AI agents | complete HTTP + curl API reference |

Both are available in Chinese and English:

```sh
curl http://localhost:5000/gofs-agent.md             # Chinese (default)
curl 'http://localhost:5000/gofs-agent.md?lang=en'   # English
```

Without `?lang=`, the `Accept-Language` request header decides; Chinese is the
fallback. Responses carry `Content-Language` and `Vary: Accept-Language`.

The agent manual is also reachable from the web UI: **Guide → Agent guide**,
which can copy the whole manual to the clipboard in one click.

## curl quick reference

```sh
# list a directory as JSON
curl "http://localhost:5000/?json="

# search recursively
curl "http://localhost:5000/?json=&q=keyword"

# download a file
curl -O http://localhost:5000/path/to/file.ext

# upload a file (parent directories are created automatically)
curl -T local/file.ext http://localhost:5000/path/to/file.ext

# create a directory
curl -X MKCOL http://localhost:5000/path/to/dir

# rename / move
curl -X MOVE -H "Destination: http://localhost:5000/new/path" http://localhost:5000/old/path

# copy
curl -X COPY -H "Destination: http://localhost:5000/new/path" http://localhost:5000/old/path

# delete (recursive for directories)
curl -X DELETE http://localhost:5000/path/to/file.ext

# download a folder as zip
curl -o dir.zip "http://localhost:5000/path/to/dir?zip"

# get the SHA256 of a file
curl "http://localhost:5000/path/to/file.ext?hash"
```

See `/gofs-agent.md` for status codes, authentication, chunked uploads and the
full parameter table.

## Layout

| File | Purpose |
|------|---------|
| `main.go` | entry point, listener setup, signal handling |
| `args.go` | CLI / env / config parsing |
| `server.go` | HTTP + WebDAV request handling, embedded asset serving |
| `docs.go` | built-in virtual Markdown documents (bilingual) |
| `auth.go` | access control, Basic/Digest auth, sha512-crypt |
| `logger.go` | structured access log (fixed fields, per-request trace) |
| `utils.go` | URI encoding, glob, range parsing, content types |
| `noscript.go` | text-client directory listing |
| `web/` | Vue 3 + TypeScript SPA source (Vite build) |
| `web/src/markdown.ts` | Markdown parser (tables, lists, math/diagram placeholders) |
| `web/src/math.ts` | KaTeX loading and math typesetting (lazy) |
| `web/src/diagram.ts` | Mermaid loading, diagram rendering and theme re-render (lazy) |
| `web/scripts/` | post-build steps (drops unused font formats) |
| `web/dist/` | built frontend, embedded into the binary with `go:embed` |
| `install.sh` | release installer (one-line install) |
| `.github/workflows/` | CI and release pipelines |
| `docs/screenshots/` | screenshots used by this README |

## CI / CD

Everything lives in `.github/workflows/`:

**`ci.yml`** runs on every push to `master` and on pull requests:

- Go: `gofmt` check, `go vet`, `go test -race`, build
- A smoke test that boots the binary and exercises `/gofs-healthz`, the JSON
  listing and both language variants of the virtual docs
- Frontend: `npm ci`, `vue-tsc --noEmit`, `npm run build`
- A guard that fails the build if `web/dist` in the repo differs from a fresh
  frontend build (the embedded assets must match the sources)

**`release.yml`** runs when you push a `v*` tag:

1. Cross-compiles with `CGO_ENABLED=0` for linux (amd64/arm64/armv6/386),
   darwin (amd64/arm64), windows (amd64/arm64) and freebsd (amd64)
2. Injects the tag into the binary (`-X main.version=<tag>`) and asserts that
   `gofs --version` reports it
3. Packages each target as `gofs_<version>_<os>_<arch>.tar.gz` (`.zip` on
   Windows) together with `LICENSE` and both READMEs
4. Generates `SHA256SUMS` and publishes a GitHub release with generated notes

Cut a release with:

```sh
git tag v0.1.2
git push origin v0.1.2
```

The installer reads that release, so the one-line install command always
follows the newest tag.

## License

MIT — see [LICENSE](LICENSE). Derived from [dufs](https://github.com/sigoden/dufs).
