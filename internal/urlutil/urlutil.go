// Package urlutil holds the URI escaping helpers shared by the HTTP layers.
package urlutil

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

// percentEncode percent-encodes every character except the unreserved set
// (A-Za-z0-9 -_.~). Each path segment is encoded independently before being
// joined.
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

// EncodeURI percent-encodes each '/' separated segment and rejoins with '/'.
func EncodeURI(v string) string {
	parts := strings.Split(v, "/")
	for i, p := range parts {
		parts[i] = percentEncode(p)
	}
	return strings.Join(parts, "/")
}

// DecodeURI percent-decodes v and reports whether the result is valid UTF-8.
func DecodeURI(v string) (string, bool) {
	s, err := url.PathUnescape(v)
	if err != nil || !utf8.ValidString(s) {
		return "", false
	}
	return s, true
}
