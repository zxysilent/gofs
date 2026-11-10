package logx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
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

// end 只带结果字段：状态码、响应体大小、耗时。
func TestAccessLogEndRecord(t *testing.T) {
	l := &AccessLogger{}
	out := captureLog(t, logs.LevelInfo, func() {
		l.End("abc12345", http.StatusOK, 260, 824*time.Microsecond, "")
	})

	for _, want := range []string{
		"level=INF",
		"trace=abc12345",
		"caller=xclose",
		"status=200",
		"size=260",
		"dur=824µs",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q: %s", want, out)
		}
	}
	// 请求信息只写在 begin，end 不重复。
	for _, unwanted := range []string{"ip=", "method=", "uri=", "user="} {
		if strings.Contains(out, unwanted) {
			t.Errorf("end record should not carry %q: %s", unwanted, out)
		}
	}
	// 旧实现会把整行包进 msg="..."，导致双重时间戳与转义引号。
	if strings.Contains(out, "msg=") {
		t.Errorf("log output should not wrap the record in msg=: %s", out)
	}
}

// begin 输出源地址 + "方法=路径"，且为 info 级别（默认可见）。
func TestAccessLogBeginRecord(t *testing.T) {
	l := &AccessLogger{}
	req := newLogRequest(t, "GET", "/a/b.md?x=1")

	out := captureLog(t, logs.LevelInfo, func() { l.Begin(req, "abc12345") })
	if out == "" {
		t.Fatal("begin should be logged at info level by default")
	}
	for _, want := range []string{
		"level=INF",
		"trace=abc12345",
		"caller=xbegin",
		"ip=192.0.2.10",
		"GET=/a/b.md?x=1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("begin output missing %q: %s", want, out)
		}
	}
	// 结果字段属于 end。
	for _, unwanted := range []string{"status=", "size=", "dur="} {
		if strings.Contains(out, unwanted) {
			t.Errorf("begin record should not carry %q: %s", unwanted, out)
		}
	}
}

// 方法作为字段名，不同请求方法各自成字段。
func TestAccessLogBeginMethodAsKey(t *testing.T) {
	l := &AccessLogger{}
	out := captureLog(t, logs.LevelInfo, func() {
		l.Begin(newLogRequest(t, "DELETE", "/gone.txt"), "del12345")
	})
	if !strings.Contains(out, "DELETE=/gone.txt") {
		t.Errorf("expected DELETE=<uri> field: %s", out)
	}
}

// 出错时用 error 级别，错误信息单独成字段。
func TestAccessLogErrorField(t *testing.T) {
	l := &AccessLogger{}
	out := captureLog(t, logs.LevelInfo, func() {
		l.End("abc12345", http.StatusInternalServerError, 0, time.Millisecond, "boom")
	})
	if !strings.Contains(out, "level=ERR") {
		t.Errorf("expected error level: %s", out)
	}
	if !strings.Contains(out, "err=boom") {
		t.Errorf("expected err field: %s", out)
	}
}

// 带认证信息的请求不会把密码写进日志。
func TestAccessLogNoCredentials(t *testing.T) {
	l := &AccessLogger{}
	req := newLogRequest(t, "GET", "/private")
	req.SetBasicAuth("alice", "secret")

	out := captureLog(t, logs.LevelInfo, func() { l.Begin(req, "abc12345") })
	if !strings.Contains(out, "caller=xbegin") {
		t.Fatalf("expected a begin record: %s", out)
	}
	if strings.Contains(out, "secret") || strings.Contains(out, "alice") {
		t.Errorf("credentials leaked into log: %s", out)
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
	req.Header.Set(RequestIDHeader, "upstream123")
	if got := RequestTrace(req); got != "upstream123" {
		t.Errorf("RequestTrace = %q, want upstream123", got)
	}

	req.Header.Set(RequestIDHeader, "bad id")
	got := RequestTrace(req)
	if got == "bad id" || len(got) == 0 {
		t.Errorf("RequestTrace should generate a new id, got %q", got)
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
