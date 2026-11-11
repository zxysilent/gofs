package server

// 访问日志的行为测试：需要真实的 Server 才能覆盖到"静态资源不记日志"和
// "trace 与响应头一致"这两条，所以放在 server 包里，logx 只测自己那部分。

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zxysilent/gofs/internal/logx"
	"github.com/zxysilent/logs"
)

// captureLog 把日志重定向到缓冲区后执行 fn，返回写入内容。
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

// nil 表示关闭访问日志（测试使用的开关）。
func TestAccessLogDisabledByServer(t *testing.T) {
	args := testArgs(t, t.TempDir(), false)
	args.AccessLog = nil
	srv := newTestServer(t, args)
	defer srv.Close()

	out := captureLog(t, logs.LevelDebug, func() {
		res, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		res.Body.Close()
	})
	if strings.Contains(out, "caller=xclose") || strings.Contains(out, "caller=xbegin") {
		t.Errorf("access log should be disabled: %s", out)
	}
}

// 正常请求会带上 X-Request-ID 响应头，且与日志中的 trace 一致。
func TestAccessLogTraceMatchesHeader(t *testing.T) {
	args := testArgs(t, t.TempDir(), false)
	args.AccessLog = &logx.AccessLogger{} // testArgs 默认关闭，这里打开
	srv := newTestServer(t, args)
	defer srv.Close()

	var header string
	out := captureLog(t, logs.LevelInfo, func() {
		res, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		header = res.Header.Get(logx.RequestIDHeader)
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
	s, err := New(args, testAssets(t), "test", &running)
	if err != nil {
		t.Fatalf("New: %v", err)
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
