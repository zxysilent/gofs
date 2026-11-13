package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var errNoFileName = errors.New("failed to get file name")

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
			// 后缀形式 "-N"：取最后 N 个字节。N 为 0 是非法（RFC 7233 §2.1），
			// N 大于文件长度则取整个文件。
			offset, err := strconv.ParseUint(end, 10, 64)
			if err != nil || offset == 0 || size == 0 {
				return nil
			}
			if offset > size {
				offset = size
			}
			result = append(result, [2]uint64{size - offset, size - 1})
		} else {
			s, err := strconv.ParseUint(start, 10, 64)
			if err != nil || s >= size {
				return nil
			}
			if end == "" {
				result = append(result, [2]uint64{s, size - 1})
				continue
			}
			e, err := strconv.ParseUint(end, 10, 64)
			if err != nil || s > e {
				return nil
			}
			// last-byte-pos 超出文件末尾时按 RFC 裁剪到最后一个字节，
			// 而不是整段拒绝 —— 否则 "bytes=0-1048575" 这种常见写法在
			// 小文件上会拿到 416 而不是文件内容。
			if e >= size {
				e = size - 1
			}
			result = append(result, [2]uint64{s, e})
		}
	}
	return result
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
//
// 文本一律标 charset=utf-8，不再嗅探具体编码：站点以 UTF-8 为准，嗅探
// （尤其是只看前 1 KiB 就猜）反而会把合法 UTF-8 判成 windows-1252，
// 让浏览器按西文解码，中文全乱码。非 UTF-8 的旧文件可能显示成乱码，
// 但那是文件本身需要转码，不应该让正常文件背锅。
func getContentType(filePath string) string {
	mimeType := mime.TypeByExtension(strings.ToLower(path.Ext(filePath)))
	// mime.TypeByExtension 对 .txt/.html 等已经带 charset，直接再拼一个就会出现
	// "text/plain; charset=utf-8; charset=utf-8"。这里先把旧参数摘掉。
	mimeType, _, _ = strings.Cut(mimeType, ";")
	f, err := os.Open(filePath)
	if err != nil {
		return fallbackMime(mimeType)
	}
	defer f.Close()
	buf := make([]byte, 1024)
	n, _ := io.ReadFull(f, buf)
	// 只要看着像文本就给 utf-8，不再区分是否「合法」：一个被切在第 1024 字节
	// 中间的多字节字符不该改变整个文件的编码声明。
	if isText(buf[:n]) {
		if mimeType != "" {
			return mimeType + "; charset=utf-8"
		}
		return "text/plain; charset=utf-8"
	}
	return fallbackMime(mimeType)
}

func fallbackMime(mimeType string) string {
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
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
