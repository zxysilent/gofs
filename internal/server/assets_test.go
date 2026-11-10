package server

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// 前端产物里对静态资源的引用保留着构建期占位符（Vite 的 base）。
// index.html 会在 renderApp 中被替换成真实前缀，但按需加载的 chunk 与
// CSS 里的 @font-face url() 无法替换，所以服务端必须把占位符当作真实前缀
// 的别名 —— 否则 KaTeX 的样式与字体 404，公式排版失效。
func TestServerAssetsPlaceholderAlias(t *testing.T) {
	args := testArgs(t, t.TempDir(), false)
	var running atomic.Bool
	running.Store(true)
	srv, err := New(args, assetsDir(t), "test", &running)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	paths := []string{
		"/" + assetsPlaceholder + "index.html",
		"/" + srv.assetsPrefix + "index.html",
	}
	for _, p := range paths {
		res, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", p, res.StatusCode)
		}
	}

	// 别名不能让不存在的资源变成 200。
	missing := "/" + assetsPlaceholder + "no-such-asset.js"
	res, err := http.Get(ts.URL + missing)
	if err != nil {
		t.Fatalf("GET %s: %v", missing, err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("GET %s status = %d, want 404", missing, res.StatusCode)
	}
}
