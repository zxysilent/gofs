package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const bufSize = 65536

var errNoFileName = errors.New("failed to get file name")

// unixNowMs returns the current Unix time in milliseconds.
func unixNowMs() int64 {
	return time.Now().UnixMilli()
}

// percentEncode percent-encodes every character except the unreserved set
// (A-Za-z0-9 -_.~), matching the Rust `urlencoding::encode` behaviour used by
// dufs. Each path segment is encoded independently before being joined.
func percentEncode(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hexDigits[c>>4])
			b.WriteByte(hexDigits[c&0xf])
		}
	}
	return b.String()
}

// encodeURI percent-encodes each '/' separated segment and rejoins with '/'.
func encodeURI(v string) string {
	parts := strings.Split(v, "/")
	for i, p := range parts {
		parts[i] = percentEncode(p)
	}
	return strings.Join(parts, "/")
}

// decodeURI percent-decodes v and reports whether the result is valid UTF-8.
func decodeURI(v string) (string, bool) {
	s, err := url.PathUnescape(v)
	if err != nil || !utf8.ValidString(s) {
		return "", false
	}
	return s, true
}

// getFileName returns the final path component ("" when there is none).
func getFileName(p string) string {
	if p == "" || p == "/" || p == "." {
		return ""
	}
	return path.Base(p)
}

// tryGetFileName returns the final path component or an error.
func tryGetFileName(p string) (string, error) {
	name := getFileName(p)
	if name == "" {
		return "", errNoFileName
	}
	return name, nil
}

// glob reports whether target matches the glob pattern. Invalid patterns never
// match. Uses path.Match syntax ('*', '?', '[...]').
func glob(pattern, target string) bool {
	ok, err := path.Match(pattern, target)
	if err != nil {
		return false
	}
	return ok
}

// parseRange parses an HTTP "bytes" Range header value. It returns nil when the
// range is invalid or unsatisfiable.
func parseRange(rng string, size uint64) [][2]uint64 {
	unit, ranges, ok := strings.Cut(rng, "=")
	if !ok || unit != "bytes" {
		return nil
	}
	var result [][2]uint64
	for _, r := range strings.Split(ranges, ",") {
		r = strings.TrimSpace(r)
		start, end, ok := strings.Cut(r, "-")
		if !ok {
			return nil
		}
		if start == "" {
			offset, err := strconv.ParseUint(end, 10, 64)
			if err != nil {
				return nil
			}
			if offset <= size {
				result = append(result, [2]uint64{size - offset, size - 1})
			} else {
				return nil
			}
		} else {
			s, err := strconv.ParseUint(start, 10, 64)
			if err != nil {
				return nil
			}
			if s >= size {
				return nil
			}
			if end == "" {
				result = append(result, [2]uint64{s, size - 1})
			} else {
				e, err := strconv.ParseUint(end, 10, 64)
				if err != nil {
					return nil
				}
				if e < size && s <= e {
					result = append(result, [2]uint64{s, e})
				} else {
					return nil
				}
			}
		}
	}
	return result
}

// isIPv6Available reports whether an IPv6 TCP socket can be created.
func isIPv6Available() bool {
	l, err := net.Listen("tcp6", "[::]:0")
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// isText reports whether a byte slice looks like text content (no NUL bytes and
// a valid text encoding).
func isText(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	if utf8.Valid(b) {
		return true
	}
	// Non-UTF-8 but may still be text in a legacy encoding: allow if mostly
	// printable ASCII / control chars that are typical of text.
	printable := 0
	for _, c := range b {
		if c >= 0x20 && c <= 0x7E || c == '\t' || c == '\n' || c == '\r' || c >= 0x80 {
			printable++
		}
	}
	return printable*100/len(b) >= 95
}

// getContentType determines a file's content type, appending a charset when the
// content looks like text.
func getContentType(filePath string) string {
	mimeType := mime.TypeByExtension(strings.ToLower(path.Ext(filePath)))
	f, err := os.Open(filePath)
	if err != nil {
		return fallbackMime(mimeType)
	}
	defer f.Close()
	buf := make([]byte, 1024)
	n, _ := io.ReadFull(f, buf)
	buf = buf[:n]
	if isText(buf) {
		charset := "utf-8"
		if !utf8.Valid(buf) {
			charset = detectCharset(buf)
		}
		if mimeType != "" {
			return mimeType + "; charset=" + charset
		}
		return "text/plain; charset=" + charset
	}
	return fallbackMime(mimeType)
}

func fallbackMime(mimeType string) string {
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

// detectCharset performs a minimal charset detection for non-UTF-8 text.
func detectCharset(b []byte) string {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return "utf-8"
	}
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		return "utf-16le"
	}
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		return "utf-16be"
	}
	return "windows-1252"
}

// sha256File returns the lowercase hex sha256 of a file.
func sha256File(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// toTimestamp converts a time.Time to milliseconds since the Unix epoch.
func toTimestamp(t time.Time) uint64 {
	return uint64(t.UnixMilli())
}
