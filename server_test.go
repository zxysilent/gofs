package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func newTestServer(t *testing.T, args Args) *httptest.Server {
	t.Helper()
	var running atomic.Bool
	running.Store(true)
	srv, err := newServer(args, &running)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	return httptest.NewServer(srv)
}

func testArgs(t *testing.T, dir string, allowAll bool) Args {
	t.Helper()
	args := defaultArgs()
	args.servePath = dir
	args.port = 0
	args.allowAll = allowAll
	args.allowUpload = allowAll
	args.allowDelete = allowAll
	args.allowSearch = allowAll
	args.allowArchive = allowAll
	args.allowHash = allowAll
	args.httpLogger = parseHttpLogger("") // disable access log
	return args
}

func TestServerReadOps(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "nested.txt"), []byte("nested"), 0o644)

	ts := newTestServer(t, testArgs(t, dir, true))
	defer ts.Close()

	// health
	resp := get(t, ts.URL+"/"+healthCheckPath)
	if resp != `{"status":"OK"}` {
		t.Errorf("health = %q", resp)
	}

	// simple listing
	resp = get(t, ts.URL+"/?simple")
	if !strings.Contains(resp, "hello.txt") || !strings.Contains(resp, "sub/") {
		t.Errorf("simple listing missing entries: %q", resp)
	}

	// json listing
	resp = get(t, ts.URL+"/?json")
	var data IndexData
	if err := json.Unmarshal([]byte(resp), &data); err != nil {
		t.Fatalf("json listing parse: %v (%q)", err, resp)
	}
	if data.Kind != "Index" || len(data.Paths) != 2 {
		t.Errorf("json listing: kind=%s paths=%d", data.Kind, len(data.Paths))
	}

	// file download
	resp = get(t, ts.URL+"/hello.txt")
	if resp != "hello world" {
		t.Errorf("download = %q", resp)
	}

	// range
	body, status := getStatus(t, ts.URL+"/hello.txt", map[string]string{"Range": "bytes=0-4"})
	if status != 206 || body != "hello" {
		t.Errorf("range: status=%d body=%q", status, body)
	}

	// html listing (SPA root with injected data)
	resp = get(t, ts.URL+"/")
	if !strings.Contains(resp, `id="app"`) {
		t.Errorf("html listing should contain the SPA mount point")
	}
	if strings.Contains(resp, "__GOFS_DATA_VALUE__") {
		t.Errorf("html listing should have injected initial data (placeholder replaced)")
	}
	if !strings.Contains(resp, "window.__GOFS_DATA__") {
		t.Errorf("html listing should expose the data global")
	}
}

func TestServerWriteOps(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, testArgs(t, dir, true))
	defer ts.Close()

	// upload
	status := put(t, ts.URL+"/upload.txt", "content")
	if status != 201 {
		t.Errorf("PUT status = %d", status)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "upload.txt")); string(b) != "content" {
		t.Errorf("uploaded file content mismatch: %q", b)
	}

	// mkcol
	status = request(t, "MKCOL", ts.URL+"/newdir", "", nil)
	if status != 201 {
		t.Errorf("MKCOL status = %d", status)
	}
	if fi, err := os.Stat(filepath.Join(dir, "newdir")); err != nil || !fi.IsDir() {
		t.Errorf("mkcol dir not created: %v", err)
	}

	// delete
	status = request(t, "DELETE", ts.URL+"/upload.txt", "", nil)
	if status != 204 {
		t.Errorf("DELETE status = %d", status)
	}
	if _, err := os.Stat(filepath.Join(dir, "upload.txt")); !os.IsNotExist(err) {
		t.Errorf("file not deleted")
	}
}

func TestServerZip(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("bbb"), 0o644)

	ts := newTestServer(t, testArgs(t, dir, true))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/?zip")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["a.txt"] || !names["sub/b.txt"] {
		t.Errorf("zip contents missing: %v", names)
	}
}

func TestServerAuthBasic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("top secret"), 0o644)

	args := testArgs(t, dir, true)
	args.auth = mustAccessControl([]string{"admin:123@/:rw"})
	ts := newTestServer(t, args)
	defer ts.Close()

	// no auth -> 401
	if status := request(t, "GET", ts.URL+"/secret.txt", "", nil); status != 401 {
		t.Errorf("unauthenticated status = %d, want 401", status)
	}

	// wrong password -> 401
	if status := request(t, "GET", ts.URL+"/secret.txt", "", map[string]string{"Authorization": "Basic " + b64("admin:wrong")}); status != 401 {
		t.Errorf("wrong password status = %d, want 401", status)
	}

	// correct -> 200
	body, status := getStatus(t, ts.URL+"/secret.txt", map[string]string{"Authorization": "Basic " + b64("admin:123")})
	if status != 200 || body != "top secret" {
		t.Errorf("auth download: status=%d body=%q", status, body)
	}
}

func TestServerAuthHashed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("top secret"), 0o644)

	args := testArgs(t, dir, true)
	args.auth = mustAccessControl([]string{"admin:$6$tWMB51u6Kb2ui3wd$5gVHP92V9kZcMwQeKTjyTRgySsYJu471Jb1I6iHQ8iZ6s07GgCIO69KcPBRuwPE5tDq05xMAzye0NxVKuJdYs/@/:rw"})
	ts := newTestServer(t, args)
	defer ts.Close()

	body, status := getStatus(t, ts.URL+"/secret.txt", map[string]string{"Authorization": "Basic " + b64("admin:123456")})
	if status != 200 || body != "top secret" {
		t.Errorf("hashed auth download: status=%d body=%q", status, body)
	}
}

func TestServerSearch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "findme.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("y"), 0o644)

	ts := newTestServer(t, testArgs(t, dir, true))
	defer ts.Close()

	resp := get(t, ts.URL+"/?q=findme&json")
	var data IndexData
	if err := json.Unmarshal([]byte(resp), &data); err != nil {
		t.Fatalf("search json: %v", err)
	}
	if len(data.Paths) != 1 || data.Paths[0].Name != "findme.txt" {
		t.Errorf("search results: %+v", data.Paths)
	}
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func get(t *testing.T, url string) string {
	t.Helper()
	body, _ := getStatus(t, url, nil)
	return body
}

func getStatus(t *testing.T, url string, headers map[string]string) (string, int) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode
}

func put(t *testing.T, url, body string) int {
	t.Helper()
	req, _ := http.NewRequest("PUT", url, strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func request(t *testing.T, method, url, body string, headers map[string]string) int {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, url, rd)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestServerVirtualDocs(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, testArgs(t, dir, true))
	defer ts.Close()

	base := ts.URL

	// 两个虚拟文件均可访问，且类型为 Markdown。
	for _, name := range []string{readmeDocPath, agentDocPath} {
		resp, err := http.Get(base + "/" + name)
		if err != nil {
			t.Fatalf("GET %s: %v", name, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("%s status = %d, want 200", name, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
			t.Errorf("%s content-type = %q", name, ct)
		}
		if !strings.Contains(string(body), base) {
			t.Errorf("%s should embed the request base URL %q", name, base)
		}
	}

	// readme 指向 agent 文档；agent 文档包含 curl 示例。
	readme := get(t, base+"/"+readmeDocPath)
	if !strings.Contains(readme, agentDocPath) {
		t.Errorf("readme should link to %s", agentDocPath)
	}
	agent := get(t, base+"/"+agentDocPath)
	if !strings.Contains(agent, "curl -T") {
		t.Errorf("agent doc should contain upload example")
	}

	// 已下线的兼容别名应返回 404（不被当作虚拟文件，也未落盘）。
	if status := request(t, "GET", base+"/gofs-curl.md", "", nil); status != 404 {
		t.Errorf("removed alias status = %d, want 404", status)
	}

	// 虚拟文件不出现在目录列表中。
	var data IndexData
	if err := json.Unmarshal([]byte(get(t, base+"/?json=")), &data); err != nil {
		t.Fatalf("list json: %v", err)
	}
	for _, p := range data.Paths {
		if _, ok := virtualDocs[p.Name]; ok {
			t.Errorf("virtual doc %q should not be listed", p.Name)
		}
	}
}

func TestServerVirtualDocsRealFileWins(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, readmeDocPath), []byte("# custom readme"), 0o644)

	ts := newTestServer(t, testArgs(t, dir, true))
	defer ts.Close()

	if body := get(t, ts.URL+"/"+readmeDocPath); body != "# custom readme" {
		t.Errorf("real file should take precedence, got %q", body)
	}
	// 其它虚拟文件仍然由服务生成。
	if body := get(t, ts.URL+"/"+agentDocPath); !strings.Contains(body, "curl -T") {
		t.Errorf("agent doc should still be generated")
	}
}

func TestServerVirtualDocsHead(t *testing.T) {
	dir := t.TempDir()
	ts := newTestServer(t, testArgs(t, dir, true))
	defer ts.Close()

	resp, err := http.Head(ts.URL + "/" + agentDocPath)
	if err != nil {
		t.Fatalf("HEAD: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("HEAD status = %d, want 200", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("HEAD should have empty body, got %d bytes", len(body))
	}
	if resp.Header.Get("Content-Length") == "0" || resp.Header.Get("Content-Length") == "" {
		t.Errorf("HEAD should report Content-Length")
	}
}

func TestServerVirtualDocsRequireAuth(t *testing.T) {
	dir := t.TempDir()
	args := testArgs(t, dir, true)
	args.auth = mustAccessControl([]string{"admin:123@/:rw"})
	ts := newTestServer(t, args)
	defer ts.Close()

	// 匿名不可读（虚拟文件同样受鉴权保护）。
	for _, name := range []string{readmeDocPath, agentDocPath} {
		if status := request(t, "GET", ts.URL+"/"+name, "", nil); status != 401 {
			t.Errorf("anonymous %s status = %d, want 401", name, status)
		}
	}

	// 认证后可读。
	body, status := getStatus(t, ts.URL+"/"+agentDocPath, map[string]string{"Authorization": "Basic " + b64("admin:123")})
	if status != 200 || !strings.Contains(body, "curl -T") {
		t.Errorf("authenticated: status=%d body=%q", status, body)
	}
}
