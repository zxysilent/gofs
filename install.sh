#!/usr/bin/env bash
#
# gofs installer — downloads a prebuilt release binary for this platform.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/zxysilent/gofs/master/install.sh | bash
#   bash -c "$(curl -fsSL https://raw.githubusercontent.com/zxysilent/gofs/master/install.sh)"
#   curl -fsSL .../install.sh | bash -s -- --version 0.1.0
#   curl -fsSL .../install.sh | bash -s -- --dir "$HOME/.local/bin"
#
# Options:
#   -v, --version <ver>   Install a specific version (default: latest release)
#   -d, --dir <path>      Install directory (default: /usr/local/bin, or $GOFS_INSTALL_DIR)
#       --dry-run         Resolve and download, but do not install
#   -h, --help            Show this help
#
# Environment:
#   GOFS_INSTALL_DIR      Same as --dir
#   GOFS_REPO             Override owner/repo (default: zxysilent/gofs)
#   GOFS_BASE_URL         Override the download base URL (mirrors / air-gapped)
#   GOFS_VERIFY           Set to 0 to skip the SHA256 check
#
set -euo pipefail

REPO="${GOFS_REPO:-zxysilent/gofs}"
VERSION=""
INSTALL_DIR="${GOFS_INSTALL_DIR:-/usr/local/bin}"
BASE_URL="${GOFS_BASE_URL:-}"
VERIFY="${GOFS_VERIFY:-1}"
DRY_RUN=0

# ---- output helpers -------------------------------------------------------

if [ -t 1 ]; then
  c_reset=$'\033[0m'; c_bold=$'\033[1m'; c_dim=$'\033[2m'
  c_green=$'\033[32m'; c_yellow=$'\033[33m'; c_red=$'\033[31m'
else
  c_reset=""; c_bold=""; c_dim=""; c_green=""; c_yellow=""; c_red=""
fi

info()  { printf '%s==>%s %s\n' "$c_bold" "$c_reset" "$*"; }
note()  { printf '%s    %s%s\n' "$c_dim" "$*" "$c_reset"; }
warn()  { printf '%swarning:%s %s\n' "$c_yellow" "$c_reset" "$*" >&2; }
die()   { printf '%serror:%s %s\n' "$c_red" "$c_reset" "$*" >&2; exit 1; }

usage() {
  # Print the leading comment block (everything before `set -euo pipefail`).
  sed -n '/^#!/d; /^#/!q; s/^# \{0,1\}//; p' "$0" 2>/dev/null || cat <<'EOF'
gofs installer — downloads a prebuilt release binary for this platform.
  -v, --version <ver>   Install a specific version (default: latest)
  -d, --dir <path>      Install directory (default: /usr/local/bin)
      --dry-run         Resolve and download, but do not install
EOF
}

# ---- argument parsing -----------------------------------------------------

while [ $# -gt 0 ]; do
  case "$1" in
    -v|--version) VERSION="${2:-}"; shift 2 ;;
    -d|--dir)     INSTALL_DIR="${2:-}"; shift 2 ;;
    --dry-run)    DRY_RUN=1; shift ;;
    -h|--help)    usage; exit 0 ;;
    *)            die "unknown argument: $1 (try --help)" ;;
  esac
done

# ---- platform detection ---------------------------------------------------

detect_os() {
  case "$(uname -s)" in
    Linux)                 echo linux ;;
    Darwin)                echo darwin ;;
    FreeBSD)               echo freebsd ;;
    MINGW*|MSYS*|CYGWIN*)
      die "Windows is not supported by this script; download the .zip from
     https://github.com/${REPO}/releases and add gofs.exe to your PATH." ;;
    *) die "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  # Normalise the many ways a machine reports its architecture to the names
  # used in the release archive file names.
  case "$(uname -m)" in
    x86_64|amd64)   echo amd64 ;;
    aarch64|arm64)  echo arm64 ;;
    armv8l|armv8)   echo arm64 ;;
    armv7l|armv7|armv6l|armv6|arm)
      # The release ships a GOARM=6 build, which also runs on ARMv7 hosts.
      echo armv6 ;;
    i386|i486|i586|i686|x86)
      echo 386 ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
}

# ---- http helpers ---------------------------------------------------------

have() { command -v "$1" >/dev/null 2>&1; }

# fetch <url> <output-path>
fetch() {
  local url="$1" out="$2"
  if have curl; then
    curl -fsSL --retry 3 --retry-delay 1 -o "$out" "$url"
  elif have wget; then
    wget -q -O "$out" "$url"
  else
    die "neither curl nor wget is available; install one of them and retry"
  fi
}

# resolve_latest_tag prints the newest release tag (e.g. v0.1.0) without using
# the JSON API, to stay well clear of anonymous rate limits.
resolve_latest_tag() {
  local url="https://github.com/${REPO}/releases/latest" location=""

  if have curl; then
    location="$(curl -fsSI -o /dev/null -w '%{url_effective}' -L "$url" 2>/dev/null || true)"
  elif have wget; then
    location="$(wget -q -S --max-redirect=0 --spider "$url" 2>&1 \
      | sed -n 's/.*[Ll]ocation: *//p' | tr -d '\r' | head -n1 || true)"
  fi

  # Both branches end up with something like ".../releases/tag/v0.1.0".
  case "$location" in
    */releases/tag/*) printf '%s\n' "${location##*/tag/}" ;;
    *) return 1 ;;
  esac
}

# ---- main -----------------------------------------------------------------

main() {
  local os arch tag ver archive base tmp bin target

  os="$(detect_os)"
  arch="$(detect_arch)"
  info "platform: ${os}/${arch}"

  if [ -n "$VERSION" ]; then
    tag="v${VERSION#v}"
  else
    tag="$(resolve_latest_tag)" || die "could not determine the latest release of ${REPO}"
  fi
  ver="${tag#v}"
  info "version:  ${ver}"

  archive="gofs_${ver}_${os}_${arch}.tar.gz"
  if [ -n "$BASE_URL" ]; then
    base="${BASE_URL%/}/${tag}"
  else
    base="https://github.com/${REPO}/releases/download/${tag}"
  fi
  info "asset:    ${archive}"

  tmp="$(mktemp -d)"
  # shellcheck disable=SC2064
  trap "rm -rf '$tmp'" EXIT

  info "downloading"
  fetch "${base}/${archive}" "${tmp}/${archive}" \
    || die "download failed: ${base}/${archive}
     Check that version ${ver} exists and has a ${os}/${arch} build:
     https://github.com/${REPO}/releases"

  if [ "$VERIFY" = "1" ]; then
    if fetch "${base}/SHA256SUMS" "${tmp}/SHA256SUMS" 2>/dev/null; then
      note "verifying checksum"
      ( cd "$tmp" && grep " ${archive}\$" SHA256SUMS > expected.sha256 && sha256sum -c expected.sha256 >/dev/null ) \
        || die "checksum verification failed for ${archive}"
      note "checksum ok"
    else
      warn "SHA256SUMS not available for ${tag}; skipping verification"
    fi
  fi

  info "extracting"
  tar -xzf "${tmp}/${archive}" -C "$tmp"

  bin="gofs"
  [ -f "${tmp}/gofs.exe" ] && bin="gofs.exe"
  [ -f "${tmp}/${bin}" ] || die "archive did not contain a gofs binary"

  note "reported version: $("${tmp}/${bin}" --version 2>/dev/null || echo unknown)"

  if [ "$DRY_RUN" = "1" ]; then
    info "dry run: would install to ${INSTALL_DIR}/${bin}"
    return 0
  fi

  target="${INSTALL_DIR}/${bin}"
  mkdir -p "$INSTALL_DIR" 2>/dev/null || true

  if [ -w "$INSTALL_DIR" ]; then
    install -m 0755 "${tmp}/${bin}" "$target" 2>/dev/null \
      || { cp "${tmp}/${bin}" "$target" && chmod 0755 "$target"; }
  else
    # Escalating privileges is the one thing we cannot do non-interactively.
    if have sudo && sudo -n true 2>/dev/null; then
      note "installing with sudo (passwordless)"
      sudo install -m 0755 "${tmp}/${bin}" "$target"
    else
      die "${INSTALL_DIR} is not writable and sudo needs a password.

     Either re-run with a writable directory:
       ... | bash -s -- --dir \"\$HOME/.local/bin\"
     or install manually:
       sudo install -m 0755 ${tmp}/${bin} ${target}
     (the extracted binary is at ${tmp}/${bin})"
    fi
  fi

  info "installed ${target}"
  if have "$bin" || command -v gofs >/dev/null 2>&1; then
    note "run 'gofs --version' to confirm"
  else
    warn "${INSTALL_DIR} is not on your PATH; add it or call ${target} directly"
  fi
}

main "$@"
