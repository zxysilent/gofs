package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- 条件请求 ----

// HTTP 日期只精确到秒，文件 mtime 带纳秒。若不截断，
// "If-Modified-Since: <Last-Modified>" 永远判为"已修改"，缓存与断点续传全部失效。
func TestConditionalRequestsWithExactLastModified(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)

	args := testArgs(t, dir, true)
	ts := newTestServer(t, args)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	lm := res.Header.Get("Last-Modified")
	etag := res.Header.Get("ETag")
	if lm == "" || etag == "" {
		t.Fatalf("missing validators: lm=%q etag=%q", lm, etag)
	}

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"If-Modified-Since = Last-Modified", map[string]string{"If-Modified-Since": lm}, http.StatusNotModified},
		{"If-Unmodified-Since = Last-Modified", map[string]string{"If-Unmodified-Since": lm}, http.StatusOK},
		{"If-None-Match = ETag", map[string]string{"If-None-Match": etag}, http.StatusNotModified},
		{"If-Match = ETag", map[string]string{"If-Match": etag}, http.StatusOK},
		{"If-Range = Last-Modified", map[string]string{"Range": "bytes=0-1", "If-Range": lm}, http.StatusPartialContent},
		{"If-Range = ETag", map[string]string{"Range": "bytes=0-1", "If-Range": etag}, http.StatusPartialContent},
		{"If-Unmodified-Since 早于 mtime", map[string]string{"If-Unmodified-Since": "Mon, 01 Jan 1990 00:00:00 GMT"}, http.StatusPreconditionFailed},
		{"If-Modified-Since 早于 mtime", map[string]string{"If-Modified-Since": "Mon, 01 Jan 1990 00:00:00 GMT"}, http.StatusOK},
	}
	for _, c := range cases {
		req, _ := http.NewRequest("GET", ts.URL+"/a.txt", nil)
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		res.Body.Close()
		if res.StatusCode != c.want {
			t.Errorf("%s: status = %d, want %d", c.name, res.StatusCode, c.want)
		}
	}
}

// ---- HEAD 与 GET 一致 ----

// HEAD 的 Content-Length 必须等于同一路径 GET 的实际长度；目录页过去因为
// HEAD 跳过列举而报出一个偏小的值。
func TestHeadMatchesGet(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world"), 0o644)

	args := testArgs(t, dir, true)
	ts := newTestServer(t, args)
	defer ts.Close()

	for _, p := range []string{"/", "/sub", "/a.txt", "/?simple"} {
		body, getStatus := getStatus(t, ts.URL+p, nil)
		if getStatus != 200 {
			t.Fatalf("GET %s status = %d", p, getStatus)
		}
		req, _ := http.NewRequest("HEAD", ts.URL+p, nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("HEAD %s: %v", p, err)
		}
		res.Body.Close()
		headLen := res.Header.Get("Content-Length")
		if want := itoa(len(body)); headLen != want {
			t.Errorf("HEAD %s Content-Length = %s, want %s (GET body %d bytes)", p, headLen, want, len(body))
		}
	}
}

// ---- Range 裁剪 ----

// last-byte-pos 超过文件末尾时按 RFC 7233 裁剪，而不是整段拒绝。
func TestRangeClampingOverHTTP(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ten.txt"), []byte("0123456789"), 0o644)

	args := testArgs(t, dir, true)
	ts := newTestServer(t, args)
	defer ts.Close()

	body, status := getStatus(t, ts.URL+"/ten.txt", map[string]string{"Range": "bytes=0-99"})
	if status != http.StatusPartialContent {
		t.Fatalf("bytes=0-99 status = %d, want 206", status)
	}
	if body != "0123456789" {
		t.Errorf("bytes=0-99 body = %q, want the whole file", body)
	}

	if _, status := getStatus(t, ts.URL+"/ten.txt", map[string]string{"Range": "bytes=-0"}); status != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("bytes=-0 status = %d, want 416", status)
	}

	if _, status := getStatus(t, ts.URL+"/ten.txt", map[string]string{"Range": "bytes=10-"}); status != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("bytes=10- status = %d, want 416", status)
	}
}

// ---- 搜索上限 ----

// 文档里承诺搜索每次最多返回 1024 条，实现必须真的截断。
func TestSearchResultCap(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 1100; i++ {
		name := filepath.Join(dir, "f"+itoa(i)+".lst")
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	args := testArgs(t, dir, true)
	ts := newTestServer(t, args)
	defer ts.Close()

	body, status := getStatus(t, ts.URL+"/?json&q=.lst", nil)
	if status != 200 {
		t.Fatalf("search status = %d", status)
	}
	if n := strings.Count(body, `"path_type"`); n > 1024 {
		t.Errorf("search returned %d entries, want at most 1024", n)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
