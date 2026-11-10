package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zxysilent/logs"
)

// captureLog 把日志重定向到缓冲区后执行 fn，返回写入内容。
// 执行完恢复全局输出与级别。
func captureLog(t *testing.T, level logs.Level, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	logs.SetOutput(&buf)
	logs.SetLevel(level)
	t.Cleanup(func() {
		logs.SetOutput(nil)
		logs.SetLevel(logs.LevelInfo)
	})
	fn()
	return buf.String()
}

func newLogRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	r.RemoteAddr = "192.0.2.10:54321"
	return r
}

// 访问日志必须输出结构化字段，而不是把整行塞进 msg=。
func TestAccessLogRecord(t *testing.T) {
	l := &accessLogger{}
	out := captureLog(t, logs.LevelInfo, func() {
		l.end(newLogRequest(t, "GET", "/a/b.md?x=1"), "abc12345", http.StatusOK, 260, 824*time.Microsecond, "")
	})

	for _, want := range []string{
		"level=INF",
		"trace=abc12345",
		"caller=api-end",
		"ip=192.0.2.10",
		"method=GET",
		"uri=/a/b.md?x=1",
		"status=200",
		"size=260",
		"dur=824µs",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q: %s", want, out)
		}
	}
	// 旧实现会把整行包进 msg="..."，导致双重时间戳与转义引号。
	if strings.Contains(out, "msg=") {
		t.Errorf("log output should not wrap the record in msg=: %s", out)
	}
}

// 出错时用 error 级别，错误信息单独成字段。
func TestAccessLogErrorField(t *testing.T) {
	l := &accessLogger{}
	out := captureLog(t, logs.LevelInfo, func() {
		l.end(newLogRequest(t, "GET", "/x"), "abc12345", http.StatusInternalServerError, 0, time.Millisecond, "boom")
	})
	if !strings.Contains(out, "level=ERR") {
		t.Errorf("expected error level: %s", out)
	}
	if !strings.Contains(out, "err=boom") {
		t.Errorf("expected err field: %s", out)
	}
}

// api-begin 只在 debug 级别出现，并与 api-end 共享同一个 trace。
func TestAccessLogBeginIsDebug(t *testing.T) {
	l := &accessLogger{}
	req := newLogRequest(t, "POST", "/up")

	quiet := captureLog(t, logs.LevelInfo, func() { l.begin(req, "abc12345") })
	if quiet != "" {
		t.Errorf("begin should be silent at info level, got: %s", quiet)
	}

	loud := captureLog(t, logs.LevelDebug, func() { l.begin(req, "abc12345") })
	if !strings.Contains(loud, "caller=api-begin") || !strings.Contains(loud, "trace=abc12345") {
		t.Errorf("unexpected begin record: %s", loud)
	}
	if strings.Contains(loud, "status=") {
		t.Errorf("begin record should not carry status: %s", loud)
	}
}

// 带认证信息的请求会把用户名记进 user 字段，密码不会出现。
func TestAccessLogUserField(t *testing.T) {
	l := &accessLogger{}
	req := newLogRequest(t, "GET", "/private")
	req.SetBasicAuth("alice", "secret")

	out := captureLog(t, logs.LevelInfo, func() {
		l.end(req, "abc12345", http.StatusOK, 1, time.Millisecond, "")
	})
	if !strings.Contains(out, "user=alice") {
		t.Errorf("expected user field: %s", out)
	}
	if strings.Contains(out, "secret") {
		t.Errorf("password leaked into log: %s", out)
	}
}

// 只有形如 8..64 位字母数字的上游 trace id 才会被复用。
func TestValidTraceID(t *testing.T) {
	valid := []string{"abc12345", "AbCdEfGh1234567890", strings.Repeat("a", 64)}
	for _, id := range valid {
		if !validTraceID(id) {
			t.Errorf("validTraceID(%q) = false, want true", id)
		}
	}
	invalid := []string{
		"", "short", strings.Repeat("a", 65),
		"bad id with space", "line\nbreak", "semi;colon", "under_score",
	}
	for _, id := range invalid {
		if validTraceID(id) {
			t.Errorf("validTraceID(%q) = true, want false", id)
		}
	}
}

// 请求携带合法 X-Request-ID 时复用之，否则生成新的。
func TestRequestTrace(t *testing.T) {
	req := newLogRequest(t, "GET", "/")
	req.Header.Set(requestIDHeader, "upstream123")
	if got := requestTrace(req); got != "upstream123" {
		t.Errorf("requestTrace = %q, want upstream123", got)
	}

	req.Header.Set(requestIDHeader, "bad id")
	got := requestTrace(req)
	if got == "bad id" || len(got) == 0 {
		t.Errorf("requestTrace should generate a new id, got %q", got)
	}
}

// 控制字符不能进入日志行，否则可以伪造日志或注入终端转义。
func TestSanitizeLogValue(t *testing.T) {
	got := sanitizeLogValue("a\nb\rc\x1bd")
	if strings.ContainsAny(got, "\n\r\x1b") {
		t.Errorf("control characters survived: %q", got)
	}
	// 引号与反斜杠交给日志库转义，这里保持原样。
	if got := sanitizeLogValue(`say "hi" \o/`); got != `say "hi" \o/` {
		t.Errorf("quote/backslash handling changed: %q", got)
	}
}

// 请求 URI 会被解码，中文文件名不会变成百分号序列。
func TestLogURI(t *testing.T) {
	r := newLogRequest(t, "GET", "/%E4%B8%AD%E6%96%87.md?x=1")
	got := logURI(r)
	if !strings.Contains(got, "中文.md") {
		t.Errorf("logURI = %q, want decoded Chinese name", got)
	}
	if !strings.Contains(got, "?x=1") {
		t.Errorf("logURI = %q, want query string preserved", got)
	}
}

// nil 表示关闭访问日志（测试使用的开关）。
func TestAccessLogDisabledByServer(t *testing.T) {
	args := testArgs(t, t.TempDir(), false)
	args.accessLog = nil
	srv := newTestServer(t, args)
	defer srv.Close()

	out := captureLog(t, logs.LevelDebug, func() {
		res, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		res.Body.Close()
	})
	if strings.Contains(out, "caller=api-end") || strings.Contains(out, "caller=api-begin") {
		t.Errorf("access log should be disabled: %s", out)
	}
}

// 正常请求会带上 X-Request-ID 响应头，且与日志中的 trace 一致。
func TestAccessLogTraceMatchesHeader(t *testing.T) {
	args := testArgs(t, t.TempDir(), false)
	args.accessLog = &accessLogger{} // testArgs 默认关闭，这里打开
	srv := newTestServer(t, args)
	defer srv.Close()

	var header string
	out := captureLog(t, logs.LevelInfo, func() {
		res, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		header = res.Header.Get(requestIDHeader)
		res.Body.Close()
	})
	if header == "" {
		t.Fatal("missing X-Request-ID response header")
	}
	if !strings.Contains(out, "trace="+header) {
		t.Errorf("log trace does not match header %q: %s", header, out)
	}
	if !strings.Contains(out, "status=200") {
		t.Errorf("expected status field: %s", out)
	}
}

// 静态资源不记录访问日志。
func TestAccessLogSkipsAssets(t *testing.T) {
	args := testArgs(t, t.TempDir(), false)
	var running atomic.Bool
	running.Store(true)
	s, err := newServer(args, &running)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	srv := httptest.NewServer(s)
	defer srv.Close()

	asset := "/" + s.assetsPrefix + "index.html"
	out := captureLog(t, logs.LevelDebug, func() {
		res, err := http.Get(srv.URL + asset)
		if err != nil {
			t.Fatalf("GET %s: %v", asset, err)
		}
		res.Body.Close()
	})
	if strings.Contains(out, "caller=api-") {
		t.Errorf("static asset should not be access-logged: %s", out)
	}
}
