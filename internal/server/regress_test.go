package server

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zxysilent/gofs/internal/config"
)

// ---- hasQueryFlag ----

func TestHasQueryFlag(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"zip", true},        // 只有名字
		{"zip=", true},       // 空值
		{"zip=1", true},      // 显式真值
		{"zip=true", true},   //
		{"zip=yes", true},    //
		{"zip=0", false},     // 显式关掉
		{"zip=false", false}, //
	}
	for _, c := range cases {
		q, err := url.ParseQuery(c.raw)
		if err != nil {
			t.Fatalf("ParseQuery(%q): %v", c.raw, err)
		}
		if got := hasQueryFlag(q, "zip"); got != c.want {
			t.Errorf("hasQueryFlag(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
	if hasQueryFlag(url.Values{}, "zip") {
		t.Error("an absent parameter must be false")
	}
}

// ---- content type ----

// mime.TypeByExtension 已经给 .txt 带了 charset，再拼一次会得到
// "text/plain; charset=utf-8; charset=utf-8"，浏览器会忽略整个头。
func TestGetContentTypeHasSingleCharset(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"a.txt":  "hello",
		"b.html": "<h1>hi</h1>",
		"c.css":  "body{}",
		"d.json": `{"a":1}`,
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		ct := getContentType(p)
		if n := strings.Count(ct, "charset="); n > 1 {
			t.Errorf("%s: content type %q has %d charset parameters", name, ct, n)
		}
		if !strings.HasPrefix(ct, "text/") && !strings.HasPrefix(ct, "application/") {
			t.Errorf("%s: unexpected content type %q", name, ct)
		}
	}
}

// 嗅探缓冲区只有 1024 字节，末尾可能正好把中文这种多字节字符切成两半。
// 以前会因此把合法 UTF-8 判成 windows-1252，浏览器按西文解码，中文全是
// 乱码（线上曾出现：同一目录里只有部分 Markdown 乱码）。现在文本一律 utf-8。
func TestGetContentTypeUTF8SplitAtSniffBoundary(t *testing.T) {
	dir := t.TempDir()
	// 「中」占 3 字节：341 个正好 1023 字节，第 1024 字节落在下一个字符内部。
	for name, body := range map[string]string{
		"split1.md": strings.Repeat("中", 341) + "# 标题\n正文\n",
		"split2.md": strings.Repeat("中", 340) + "ab" + "# 标题\n",
		"emojis.md": strings.Repeat("😀", 256) + "尾巴\n", // 4 字节字符，255.75 个落在界内
		"deep.txt":  strings.Repeat("汉", 500),
		"beyond.md": strings.Repeat("x", 1023) + "中文内容\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if ct := getContentType(p); !strings.Contains(ct, "charset=utf-8") {
			t.Errorf("%s: content type = %q, want charset=utf-8", name, ct)
		}
	}
}

// 非 UTF-8 的旧文件也一律标 utf-8：站点以 UTF-8 为准，不去嗅探编码。
// 需要旧编码文件的用户应当自行转码，而不是让正常文件跟着变乱码。
func TestGetContentTypeLegacyTextIsStillUTF8(t *testing.T) {
	dir := t.TempDir()
	// windows-1252 的 café：0xE9 不是合法 UTF-8 字节。
	p := filepath.Join(dir, "legacy.txt")
	if err := os.WriteFile(p, []byte("caf\xe9 na\xefve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ct := getContentType(p)
	if n := strings.Count(ct, "charset="); n != 1 {
		t.Errorf("content type %q has %d charset parameters, want 1", ct, n)
	}
	if !strings.Contains(ct, "charset=utf-8") {
		t.Errorf("content type %q: 文本应统一标 utf-8", ct)
	}
}

// ---- WebDAV root naming ----

// 服务根的相对路径是 "."，必须归一成空串，否则 PROPFIND 会返回 /./ 与 "."。
func TestPropfindRootHref(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)

	args := config.DefaultArgs()
	args.ServePath = dir
	args.AllowUpload, args.AllowDelete, args.AllowSearch, args.AllowArchive, args.AllowHash = true, true, true, true, true
	args.AccessLog = nil
	ts := newTestServer(t, args)
	defer ts.Close()

	req, err := http.NewRequest("PROPFIND", ts.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Depth", "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 207 {
		t.Fatalf("PROPFIND status = %d, want 207", res.StatusCode)
	}
	rawBody, _ := io.ReadAll(res.Body)
	body := string(rawBody)

	if strings.Contains(body, "<D:href>/./</D:href>") {
		t.Errorf("root href must not contain /./:\n%s", body)
	}
	if !strings.Contains(body, "<D:href>/</D:href>") {
		t.Errorf("root href should be /:\n%s", body)
	}
	if strings.Contains(body, "<D:displayname>.</D:displayname>") {
		t.Errorf("root displayname must not be \".\":\n%s", body)
	}
	if !strings.Contains(body, "<D:href>/a.txt</D:href>") {
		t.Errorf("child href missing:\n%s", body)
	}
}

// ---- render-spa ----

// SPA 模式缺 index.html 时是"没有可服务的内容"，不能因为 open 失败报 500。
func TestRenderSpAMissingIndexIs404(t *testing.T) {
	dir := t.TempDir() // 故意不放 index.html

	args := config.DefaultArgs()
	args.ServePath = dir
	args.RenderSpA = true
	args.AccessLog = nil
	ts := newTestServer(t, args)
	defer ts.Close()

	for _, p := range []string{"/some/deep/route", "/"} {
		res, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", p, res.StatusCode)
		}
	}
}

// 有 index.html 时，无扩展名的路径回落到 SPA，带扩展名的仍是 404。
func TestRenderSpAServesIndex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0o644)

	args := config.DefaultArgs()
	args.ServePath = dir
	args.RenderSpA = true
	args.AccessLog = nil
	ts := newTestServer(t, args)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/deep/route")
	if err != nil {
		t.Fatal(err)
	}
	rawBody, _ := io.ReadAll(res.Body)
	body := string(rawBody)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(body, "spa") {
		t.Errorf("SPA route: status=%d body=%q", res.StatusCode, body)
	}

	res2, err := http.Get(ts.URL + "/missing.js")
	if err != nil {
		t.Fatal(err)
	}
	res2.Body.Close()
	if res2.StatusCode != http.StatusNotFound {
		t.Errorf("missing asset status = %d, want 404", res2.StatusCode)
	}
}
