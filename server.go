package main

import (
	"archive/zip"
	"compress/flate"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	version                = "0.46.0"
	indexName              = "index.html"
	resumableUploadMinSize = 20971520 // 20M
	healthCheckPath        = "__dufs__/health"
	maxSubpathsCount       = 1000
	// curlManualPath is the virtual path serving the curl usage manual
	// (Markdown). A real file with the same name takes precedence.
	curlManualPath = "gofs-curl.md"
)

//go:embed web/dist
var assetsFS embed.FS

// assetsPrefixDir is the directory inside the embedded FS that holds the
// built frontend (Vue app).
const assetsPrefixDir = "web/dist"

var embeddedIndexHTML = mustAsset(assetsPrefixDir + "/index.html")

func mustAsset(name string) string {
	b, err := assetsFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(b)
}

type PathType string

const (
	pathDir         PathType = "Dir"
	pathSymlinkDir  PathType = "SymlinkDir"
	pathFile        PathType = "File"
	pathSymlinkFile PathType = "SymlinkFile"
)

func (t PathType) isDir() bool { return t == pathDir || t == pathSymlinkDir }

type PathItem struct {
	PathType PathType `json:"path_type"`
	Name     string   `json:"name"`
	Mtime    uint64   `json:"mtime"`
	Size     uint64   `json:"size"`
}

func (p *PathItem) isDir() bool { return p.PathType.isDir() }

func (p *PathItem) baseName() string {
	if idx := strings.LastIndexByte(p.Name, '/'); idx >= 0 {
		return p.Name[idx+1:]
	}
	return p.Name
}

func (p *PathItem) toDavXML(prefix string) string {
	mtime := time.UnixMilli(int64(p.Mtime)).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	href := encodeURI(prefix + p.Name)
	if p.isDir() && !strings.HasSuffix(href, "/") {
		href += "/"
	}
	displayName := escapeXML(p.baseName())
	if p.isDir() {
		return fmt.Sprintf(`<D:response>
<D:href>%s</D:href>
<D:propstat>
<D:prop>
<D:displayname>%s</D:displayname>
<D:getlastmodified>%s</D:getlastmodified>
<D:resourcetype><D:collection/></D:resourcetype>
</D:prop>
<D:status>HTTP/1.1 200 OK</D:status>
</D:propstat>
</D:response>`, href, displayName, mtime)
	}
	return fmt.Sprintf(`<D:response>
<D:href>%s</D:href>
<D:propstat>
<D:prop>
<D:displayname>%s</D:displayname>
<D:getcontentlength>%d</D:getcontentlength>
<D:getlastmodified>%s</D:getlastmodified>
<D:resourcetype></D:resourcetype>
</D:prop>
<D:status>HTTP/1.1 200 OK</D:status>
</D:propstat>
</D:response>`, href, displayName, p.Size, mtime)
}

func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

const (
	kindIndex = "Index"
)

type IndexData struct {
	Href         string     `json:"href"`
	Kind         string     `json:"kind"`
	URIPrefix    string     `json:"uri_prefix"`
	AllowUpload  bool       `json:"allow_upload"`
	AllowDelete  bool       `json:"allow_delete"`
	AllowSearch  bool       `json:"allow_search"`
	AllowArchive bool       `json:"allow_archive"`
	DirExists    bool       `json:"dir_exists"`
	Auth         bool       `json:"auth"`
	User         *string    `json:"user"`
	Paths        []PathItem `json:"paths"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type Server struct {
	args               Args
	assetsPrefix       string
	html               string
	singleFileReqPaths []string
	running            *atomic.Bool
}

func newServer(args Args, running *atomic.Bool) (*Server, error) {
	assetsPrefix := fmt.Sprintf("__dufs_v%s__/", version)
	var singleFileReqPaths []string
	if args.pathIsFile {
		singleFileReqPaths = []string{
			args.uriPrefix,
			args.uriPrefix[:len(args.uriPrefix)-1],
			encodeURI(args.uriPrefix + getFileName(args.servePath)),
		}
	}
	html := embeddedIndexHTML
	if args.assets != "" {
		b, err := os.ReadFile(filepath.Join(args.assets, indexName))
		if err != nil {
			return nil, err
		}
		html = string(b)
	}
	return &Server{
		args:               args,
		assetsPrefix:       assetsPrefix,
		html:               html,
		singleFileReqPaths: singleFileReqPaths,
		running:            running,
	}, nil
}

// responseWriter records the status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	if s.args.enableCors {
		addCors(rw)
	}
	data := s.args.httpLogger.data(r)
	data["remote_addr"] = remoteIP(r.RemoteAddr)

	err := s.handle(rw, r)
	if err != nil && !rw.wroteHeader {
		rw.WriteHeader(http.StatusInternalServerError)
		io.WriteString(rw, "Internal Server Error")
	}
	data["status"] = strconv.Itoa(rw.status)
	if !strings.HasPrefix(r.URL.Path, s.assetsPrefix) {
		if err != nil {
			s.args.httpLogger.log(data, err.Error())
		} else {
			s.args.httpLogger.log(data, "")
		}
	}
}

func (s *Server) handle(w *responseWriter, r *http.Request) error {
	reqPath := r.URL.EscapedPath()
	method := r.Method

	relativePath, ok := s.resolvePath(r.URL.Path)
	if !ok {
		statusBadRequest(w, "Invalid Path")
		return nil
	}

	// 虚拟文件：curl 使用手册（Markdown）。磁盘上存在同名文件时以真实文件为准。
	if relativePath == curlManualPath && (method == http.MethodGet || method == http.MethodHead) {
		if p, ok := s.joinPath(relativePath); !ok || !fileExists(p) {
			return s.serveCurlManual(w, r, method == http.MethodHead)
		}
	}

	if method == http.MethodGet {
		done, err := s.handleInternal(relativePath, w)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}

	userAgent := strings.ToLower(r.Header.Get("User-Agent"))
	isMicrosoftWebdav := strings.HasPrefix(userAgent, "microsoft-webdav-miniredir/")
	if isMicrosoftWebdav {
		w.Header().Set("Connection", "close")
	}

	authorization := r.Header.Get("Authorization")
	q := r.URL.Query()

	user, accessPaths := s.args.auth.guard(relativePath, method, authorization, q.Get("token"), isMicrosoftWebdav)
	if accessPaths == nil {
		if user == "" {
			s.authReject(w)
		} else {
			statusForbidden(w)
		}
		return nil
	}

	if detectNoScript(userAgent) {
		q.Set("noscript", "")
	}

	if method == "CHECKAUTH" {
		if user != "" {
			io.WriteString(w, user)
		} else {
			if hasQueryFlag(q, "login") || !accessPaths.perm.readWrite() {
				s.authReject(w)
			} else {
				io.WriteString(w, "")
			}
		}
		return nil
	}
	if method == "LOGOUT" {
		s.authReject(w)
		return nil
	}

	if hasQueryFlag(q, "tokengen") {
		return s.handleTokengen(w, relativePath, user)
	}

	headOnly := method == http.MethodHead

	if s.args.pathIsFile {
		matched := false
		for _, v := range s.singleFileReqPaths {
			if v == reqPath {
				matched = true
				break
			}
		}
		if matched {
			return s.handleSendFile(s.args.servePath, r.Header, headOnly, w)
		}
		return s.handleNotFound(q, r.Header, headOnly, w)
	}

	path, ok := s.joinPath(relativePath)
	if !ok {
		statusForbidden(w)
		return nil
	}

	info, statErr := os.Stat(path)
	isMiss := statErr != nil
	isDir, isFile, size := false, false, uint64(0)
	if !isMiss {
		isDir = info.IsDir()
		isFile = info.Mode().IsRegular()
		size = uint64(info.Size())
	}

	if s.guardRootContained(path) {
		return s.handleNotFound(q, r.Header, headOnly, w)
	}

	switch method {
	case http.MethodGet, http.MethodHead:
		if isDir {
			switch {
			case s.args.renderTryIndex:
				if s.args.allowArchive && hasQueryFlag(q, "zip") {
					return s.handleZipDir(path, headOnly, accessPaths, w)
				} else if s.args.allowSearch && q.Has("q") {
					return s.handleSearchDir(path, q, headOnly, user, accessPaths, w)
				}
				return s.handleRenderIndex(path, q, r.Header, headOnly, user, accessPaths, w)
			case s.args.renderIndex || s.args.renderSpA:
				return s.handleRenderIndex(path, q, r.Header, headOnly, user, accessPaths, w)
			case hasQueryFlag(q, "zip"):
				if !s.args.allowArchive {
					statusNotFound(w)
					return nil
				}
				return s.handleZipDir(path, headOnly, accessPaths, w)
			case s.args.allowSearch && q.Has("q"):
				return s.handleSearchDir(path, q, headOnly, user, accessPaths, w)
			default:
				return s.handleLsDir(path, true, q, headOnly, user, accessPaths, w)
			}
		} else if isFile {
			switch {
			case hasQueryFlag(q, "json"):
				return s.handleFileJSON(path, headOnly, w)
			case hasQueryFlag(q, "hash"):
				if s.args.allowHash {
					return s.handleHashFile(path, headOnly, w)
				}
				statusForbidden(w)
				return nil
			default:
				return s.handleSendFile(path, r.Header, headOnly, w)
			}
		} else if s.args.renderSpA {
			return s.handleRenderSpA(path, q, r.Header, headOnly, w)
		} else if s.args.allowUpload && strings.HasSuffix(reqPath, "/") {
			return s.handleLsDir(path, false, q, headOnly, user, accessPaths, w)
		}
		return s.handleNotFound(q, r.Header, headOnly, w)

	case http.MethodOptions:
		setWebdavHeaders(w)

	case http.MethodPut:
		if isDir || !s.args.allowUpload || (!s.args.allowDelete && size > 0) {
			statusForbidden(w)
		} else {
			return s.handleUpload(path, nil, size, r, w)
		}

	case http.MethodPatch:
		if isMiss {
			statusNotFound(w)
		} else if !s.args.allowUpload {
			statusForbidden(w)
		} else {
			offset, err := parseUploadOffset(r.Header, size)
			if err != nil {
				statusBadRequest(w, err.Error())
				return nil
			}
			if offset != nil {
				if *offset < size && !s.args.allowDelete {
					statusForbidden(w)
					return nil
				}
				return s.handleUpload(path, offset, size, r, w)
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		}

	case http.MethodDelete:
		if !s.args.allowDelete {
			statusForbidden(w)
		} else if !isMiss {
			return s.handleDelete(path, isDir, w)
		} else {
			statusNotFound(w)
		}

	default:
		switch method {
		case "PROPFIND":
			if isDir {
				ap := accessPaths
				if ap.perm.indexOnly() && authorization == "" {
					ap = newAccessPaths(permReadOnly)
				}
				return s.handlePropfindDir(path, r.Header, ap, w)
			} else if isFile {
				return s.handlePropfindFile(path, w)
			}
			statusNotFound(w)
		case "PROPPATCH":
			if isFile {
				return s.handleProppatch(reqPath, w)
			}
			statusNotFound(w)
		case "MKCOL":
			if !s.args.allowUpload {
				statusForbidden(w)
			} else if !isMiss {
				w.WriteHeader(http.StatusMethodNotAllowed)
				io.WriteString(w, "Already exists")
			} else {
				return s.handleMkcol(path, w)
			}
		case "COPY":
			if !s.args.allowUpload {
				statusForbidden(w)
			} else if isMiss {
				statusNotFound(w)
			} else {
				return s.handleCopy(path, r, w)
			}
		case "MOVE":
			if !s.args.allowUpload || !s.args.allowDelete {
				statusForbidden(w)
			} else if isMiss {
				statusNotFound(w)
			} else {
				return s.handleMove(path, r, w)
			}
		case "LOCK":
			if isFile {
				return s.handleLock(reqPath, authorization != "", w)
			}
			statusNotFound(w)
		case "UNLOCK":
			if isMiss {
				statusNotFound(w)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
	return nil
}

// ---- internal assets & health ----

// requestBaseURL returns the externally visible base URL (no trailing slash).
func requestBaseURL(r *http.Request, uriPrefix string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		if i := strings.IndexByte(proto, ','); i >= 0 {
			proto = proto[:i]
		}
		if proto = strings.TrimSpace(proto); proto != "" {
			scheme = proto
		}
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			fwd = fwd[:i]
		}
		if fwd = strings.TrimSpace(fwd); fwd != "" {
			host = fwd
		}
	}
	return strings.TrimSuffix(scheme+"://"+host+uriPrefix, "/")
}

// generateCurlManual renders the curl usage manual in Markdown for a base URL.
func generateCurlManual(base string) string {
	var b strings.Builder
	b.WriteString("# gofs curl 使用手册\n\n")
	b.WriteString("服务地址：" + base + "\n")
	b.WriteString("本手册地址：" + base + "/" + curlManualPath + "\n")

	section := func(title, body string) {
		b.WriteString("\n## " + title + "\n")
		b.WriteString(body + "\n")
	}

	section("上传文件", "curl -T local/path/to/file.ext "+base+"/path/to/file.ext")
	section("下载文件", "curl -O "+base+"/path/to/file.ext")
	section("查看列表", `curl "`+base+`/?json="`)
	section("搜索文件", `curl "`+base+`/?json=&q=keyword"`)
	section("重命名 / 移动", `curl -X MOVE -H "Destination: `+base+`/new/path" `+base+`/old/path`)
	section("复制", `curl -X COPY -H "Destination: `+base+`/new/path" `+base+`/old/path`)
	section("删除", "curl -X DELETE "+base+"/path/to/file.ext")
	section("新建目录", "curl -X MKCOL "+base+"/path/to/dir")
	section("新建空文件", `curl -X PUT --data "" `+base+`/path/to/empty.txt`)
	section("打包下载目录", `curl -o dir.zip "`+base+`/path/to/dir?zip"`)
	section("获取文件 SHA256", `curl "`+base+`/path/to/file.ext?hash"`)
	section("分片上传（大文件）", `curl -T local/path/to/file.ext -H "X-Update-Range: append" `+base+`/path/to/file.ext`)
	section("带认证访问（如启用了认证）",
		"curl -u user:pass "+base+"/path/to/file.ext\n"+
			`curl -H "Authorization: Basic $(echo -n user:pass | base64)" `+base+`/path/to/file.ext`)
	return b.String()
}

// serveCurlManual serves the generated Markdown manual.
func (s *Server) serveCurlManual(w *responseWriter, r *http.Request, headOnly bool) error {
	body := generateCurlManual(requestBaseURL(r, s.args.uriPrefix))
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if headOnly {
		return nil
	}
	_, err := io.WriteString(w, body)
	return err
}

func (s *Server) handleInternal(reqPath string, w *responseWriter) (bool, error) {
	if name, ok := strings.CutPrefix(reqPath, s.assetsPrefix); ok {
		if s.args.assets != "" {
			p := filepath.Join(s.args.assets, name)
			if fileExists(p) {
				return true, s.handleSendFile(p, nil, false, w)
			}
			statusNotFound(w)
			return true, nil
		}
		name = strings.TrimPrefix(name, "/")
		if name == "" || name == "index.html" {
			name = "index.html"
		}
		data, err := assetsFS.ReadFile(assetsPrefixDir + "/" + name)
		if err != nil {
			statusNotFound(w)
			return true, nil
		}
		w.Header().Set("Content-Type", contentTypeByName(name))
		w.Header().Set("Cache-Control", cacheControlForAsset(name))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
		return true, nil
	}
	if reqPath == healthCheckPath {
		body := `{"status":"OK"}`
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		io.WriteString(w, body)
		return true, nil
	}
	return false, nil
}

// contentTypeByName maps a static asset file name to a MIME type.
func contentTypeByName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".js", ".mjs":
		return "application/javascript; charset=UTF-8"
	case ".css":
		return "text/css; charset=UTF-8"
	case ".html":
		return "text/html; charset=UTF-8"
	case ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".map":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// cacheControlForAsset returns an appropriate Cache-Control header. Vite builds
// emit content-hashed file names (assets/index-<hash>.js|.css), so those are
// safely immutable. index.html is never hashed and must be revalidated.
func cacheControlForAsset(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".map":
		return "no-cache"
	default:
		return "public, max-age=31536000, immutable"
	}
}

// ---- path resolution ----

func (s *Server) resolvePath(p string) (string, bool) {
	p = strings.Trim(p, "/")
	var parts []string
	if p != "" {
		for _, comp := range strings.Split(p, "/") {
			if comp == "" {
				continue // collapse multiple separators
			}
			if comp == "." || comp == ".." {
				return "", false
			}
			parts = append(parts, comp)
		}
	}
	newPath := strings.Join(parts, "/")
	prefix := s.args.pathPrefix
	if prefix == "" {
		return newPath, true
	}
	if newPath == prefix {
		return "", true
	}
	if strings.HasPrefix(newPath, prefix+"/") {
		return strings.Trim(strings.TrimPrefix(newPath, prefix+"/"), "/"), true
	}
	return "", false
}

func (s *Server) joinPath(p string) (string, bool) {
	if p == "" {
		return s.args.servePath, true
	}
	return filepath.Join(s.args.servePath, p), true
}

func (s *Server) guardRootContained(path string) bool {
	if s.args.allowSymlink {
		return false
	}
	check := path
	for {
		if _, err := os.Lstat(check); err == nil {
			break
		}
		parent := filepath.Dir(check)
		if parent == check {
			return true
		}
		check = parent
	}
	return !s.isRootContained(check)
}

func (s *Server) isRootContained(path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return resolved == s.args.servePath || strings.HasPrefix(resolved, s.args.servePath+string(filepath.Separator))
}

// ---- upload / delete / mkcol / copy / move / lock ----

func (s *Server) handleUpload(path string, uploadOffset *uint64, size uint64, r *http.Request, w *responseWriter) error {
	if err := ensurePathParent(path); err != nil {
		return err
	}
	var f *os.File
	var err error
	status := http.StatusCreated
	switch {
	case uploadOffset == nil:
		f, err = os.Create(path)
	case *uploadOffset == size:
		f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		status = http.StatusNoContent
	default:
		f, err = os.OpenFile(path, os.O_WRONLY, 0)
		if err == nil {
			_, err = f.Seek(int64(*uploadOffset), io.SeekStart)
		}
		status = http.StatusNoContent
	}
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r.Body)
	f.Close()
	if copyErr != nil {
		if uploadOffset == nil && fileSize(path) < resumableUploadMinSize {
			os.Remove(path)
		}
		return copyErr
	}
	w.WriteHeader(status)
	return nil
}

func (s *Server) handleDelete(path string, isDir bool, w *responseWriter) error {
	var err error
	if isDir {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}
	if err != nil {
		return err
	}
	statusNoContent(w)
	return nil
}

func (s *Server) handleMkcol(path string, w *responseWriter) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func (s *Server) handleCopy(path string, r *http.Request, w *responseWriter) error {
	dest, ok := s.extractDest(r, w)
	if !ok {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		statusForbidden(w)
		return nil
	}
	if err := ensurePathParent(dest); err != nil {
		return err
	}
	if s.guardRootContained(dest) {
		statusBadRequest(w, "Invalid Destination")
		return nil
	}
	if err := copyFile(path, dest); err != nil {
		return err
	}
	statusNoContent(w)
	return nil
}

func (s *Server) handleMove(path string, r *http.Request, w *responseWriter) error {
	dest, ok := s.extractDest(r, w)
	if !ok {
		return nil
	}
	if err := ensurePathParent(dest); err != nil {
		return err
	}
	if s.guardRootContained(dest) {
		statusBadRequest(w, "Invalid Destination")
		return nil
	}
	if err := os.Rename(path, dest); err != nil {
		return err
	}
	statusNoContent(w)
	return nil
}

func (s *Server) handleLock(reqPath string, auth bool, w *responseWriter) error {
	token := strconv.FormatInt(time.Now().Unix(), 10)
	if auth {
		token = "opaquelocktoken:" + randomUUID()
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Lock-Token", "<"+token+">")
	io.WriteString(w, fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<D:prop xmlns:D="DAV:"><D:lockdiscovery><D:activelock>
<D:locktoken><D:href>%s</D:href></D:locktoken>
<D:lockroot><D:href>%s</D:href></D:lockroot>
</D:activelock></D:lockdiscovery></D:prop>`, token, reqPath))
	return nil
}

func (s *Server) handleProppatch(reqPath string, w *responseWriter) error {
	resMultistatus(w, fmt.Sprintf(`<D:response>
<D:href>%s</D:href>
<D:propstat>
<D:prop>
</D:prop>
<D:status>HTTP/1.1 403 Forbidden</D:status>
</D:propstat>
</D:response>`, reqPath))
	return nil
}

func (s *Server) extractDest(r *http.Request, w *responseWriter) (string, bool) {
	dest := r.Header.Get("Destination")
	u, err := url.Parse(dest)
	if err != nil {
		statusBadRequest(w, "Invalid Destination")
		return "", false
	}
	destPath := u.Path
	relative, ok := s.resolvePath(destPath)
	if !ok {
		statusBadRequest(w, "Invalid Destination")
		return "", false
	}
	_, ap := s.args.auth.guard(relative, r.Method, r.Header.Get("Authorization"), "", false)
	if ap == nil {
		statusForbidden(w)
		return "", false
	}
	d, ok := s.joinPath(relative)
	if !ok {
		statusBadRequest(w, "Invalid Destination")
		return "", false
	}
	return d, true
}

// ---- directory listing / search / zip / index ----

func (s *Server) handleLsDir(path string, exist bool, q url.Values, headOnly bool, user string, accessPaths *AccessPaths, w *responseWriter) error {
	var paths []PathItem
	if !headOnly && exist {
		var err error
		paths, err = s.listDir(path, path, accessPaths)
		if err != nil {
			statusForbidden(w)
			return nil
		}
	}
	return s.sendIndex(path, paths, exist, q, headOnly, user, accessPaths, w)
}

func (s *Server) handleSearchDir(path string, q url.Values, headOnly bool, user string, accessPaths *AccessPaths, w *responseWriter) error {
	var paths []PathItem
	search := strings.ToLower(q.Get("q"))
	if search == "" {
		return s.handleLsDir(path, true, q, headOnly, user, accessPaths, w)
	}
	if !headOnly {
		for _, sp := range s.collectDirEntries(path, accessPaths, func(p string, isFile bool) bool {
			return strings.Contains(strings.ToLower(getFileName(p)), search)
		}) {
			if item, err := s.toPathItem(sp, path); err == nil && item != nil {
				paths = append(paths, *item)
			}
		}
	}
	return s.sendIndex(path, paths, true, q, headOnly, user, accessPaths, w)
}

func (s *Server) handleZipDir(path string, headOnly bool, accessPaths *AccessPaths, w *responseWriter) error {
	filename, err := tryGetFileName(path)
	if err != nil {
		return err
	}
	setContentDisposition(w, false, filename+".zip")
	w.Header().Set("Content-Type", "application/zip")
	if headOnly {
		return nil
	}
	zw := zip.NewWriter(w)
	s.setupZipCompressor(zw)
	if err := s.zipDir(zw, path, accessPaths); err != nil {
		return err
	}
	return zw.Close()
}

func (s *Server) setupZipCompressor(zw *zip.Writer) {
	switch s.args.compress {
	case compressLow:
		zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(out, flate.BestSpeed)
		})
	case compressMedium:
		zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(out, flate.DefaultCompression)
		})
	case compressHigh:
		zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(out, flate.BestCompression)
		})
	}
}

func (s *Server) zipDir(zw *zip.Writer, dir string, accessPaths *AccessPaths) error {
	for _, file := range s.collectDirEntries(dir, accessPaths, func(p string, isFile bool) bool { return isFile }) {
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}
		hdr := &zip.FileHeader{Name: rel, Method: zip.Deflate}
		if s.args.compress == compressNone {
			hdr.Method = zip.Store
		}
		hdr.SetModTime(info.ModTime())
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			f.Close()
			return err
		}
		_, err = io.Copy(w, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleRenderIndex(path string, q url.Values, header http.Header, headOnly bool, user string, accessPaths *AccessPaths, w *responseWriter) error {
	indexPath := filepath.Join(path, indexName)
	if info, err := os.Stat(indexPath); err == nil && info.Mode().IsRegular() {
		return s.handleSendFile(indexPath, header, headOnly, w)
	}
	if s.args.renderTryIndex {
		return s.handleLsDir(path, true, q, headOnly, user, accessPaths, w)
	}
	return s.handleNotFound(q, header, headOnly, w)
}

func (s *Server) handleRenderSpA(path string, q url.Values, header http.Header, headOnly bool, w *responseWriter) error {
	if filepath.Ext(path) == "" {
		return s.handleSendFile(filepath.Join(s.args.servePath, indexName), header, headOnly, w)
	}
	return s.handleNotFound(q, header, headOnly, w)
}

func (s *Server) handleNotFound(q url.Values, header http.Header, headOnly bool, w *responseWriter) error {
	if s.args.errorPage != "" && !hasQueryFlag(q, "noscript") {
		content, err := os.ReadFile(s.args.errorPage)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", getContentType(s.args.errorPage))
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.WriteHeader(http.StatusNotFound)
		if headOnly {
			return nil
		}
		_, err = w.Write(content)
		return err
	}
	statusNotFound(w)
	return nil
}

func (s *Server) handleFileJSON(path string, headOnly bool, w *responseWriter) error {
	item, err := s.toPathItem(path, s.args.servePath)
	if err != nil || item == nil {
		statusNotFound(w)
		return nil
	}
	output, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(output)))
	if headOnly {
		return nil
	}
	_, err = w.Write(output)
	return err
}

// renderApp renders the single-page application, injecting the given data as a
// base64 JSON blob and rewriting the static asset base to the server's
// dedicated prefix.
func (s *Server) renderApp(w *responseWriter, data interface{}, headOnly bool) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)
	html := s.html
	html = strings.ReplaceAll(html, "/__GOFS_ASSETS__/", s.args.uriPrefix+s.assetsPrefix)
	html = strings.ReplaceAll(html, "__GOFS_DATA_VALUE__", encoded)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(html)))
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if headOnly {
		return nil
	}
	_, err = io.WriteString(w, html)
	return err
}

func (s *Server) handleHashFile(path string, headOnly bool, w *responseWriter) error {
	output, err := sha256File(path)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(output)))
	if headOnly {
		return nil
	}
	_, err = io.WriteString(w, output)
	return err
}

func (s *Server) handleTokengen(w *responseWriter, relativePath, user string) error {
	output, err := s.args.auth.generateToken(relativePath, user)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(output)))
	_, err = io.WriteString(w, output)
	return err
}

func (s *Server) handlePropfindDir(path string, header http.Header, accessPaths *AccessPaths, w *responseWriter) error {
	depth := 1
	if v := header.Get("Depth"); v != "" {
		switch v {
		case "0":
			depth = 0
		case "1":
			depth = 1
		default:
			statusBadRequest(w, "Invalid depth: only 0 and 1 are allowed.")
			return nil
		}
	}
	var paths []PathItem
	if item, err := s.toPathItem(path, s.args.servePath); err == nil && item != nil {
		paths = append(paths, *item)
	}
	if depth == 1 {
		if children, err := s.listDir(path, s.args.servePath, accessPaths); err == nil {
			paths = append(paths, children...)
		} else {
			statusForbidden(w)
			return nil
		}
	}
	var sb strings.Builder
	for i := range paths {
		sb.WriteString(paths[i].toDavXML(s.args.uriPrefix))
	}
	resMultistatus(w, sb.String())
	return nil
}

func (s *Server) handlePropfindFile(path string, w *responseWriter) error {
	item, err := s.toPathItem(path, s.args.servePath)
	if err != nil || item == nil {
		statusNotFound(w)
		return nil
	}
	resMultistatus(w, item.toDavXML(s.args.uriPrefix))
	return nil
}

// ---- send file ----

func (s *Server) handleSendFile(path string, header http.Header, headOnly bool, w *responseWriter) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := uint64(info.Size())

	mtime := info.ModTime()
	etag := fmt.Sprintf(`"%d-%d"`, toTimestamp(mtime), size)
	lastModified := mtime.UTC().Format(http.TimeFormat)

	useRange := false
	if header != nil {
		if ifUnmod := header.Get("If-Unmodified-Since"); ifUnmod != "" {
			if t, err := http.ParseTime(ifUnmod); err == nil && mtime.After(t) {
				w.WriteHeader(http.StatusPreconditionFailed)
				return nil
			}
		}
		if ifMatch := header.Get("If-Match"); ifMatch != "" && !etagMatches(ifMatch, etag) {
			w.WriteHeader(http.StatusPreconditionFailed)
			return nil
		}
		if ifMod := header.Get("If-Modified-Since"); ifMod != "" {
			if t, err := http.ParseTime(ifMod); err == nil && !mtime.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return nil
			}
		}
		if ifNone := header.Get("If-None-Match"); ifNone != "" && etagMatches(ifNone, etag) {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}

		rangeHeader := header.Get("Range")
		if rangeHeader != "" {
			if ifRange := header.Get("If-Range"); ifRange != "" {
				if strings.HasPrefix(ifRange, `"`) || strings.HasPrefix(ifRange, "W/") {
					useRange = etagMatches(ifRange, etag)
				} else if t, err := http.ParseTime(ifRange); err == nil {
					useRange = !mtime.After(t)
				}
			} else {
				useRange = true
			}
		}
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Last-Modified", lastModified)
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", getContentType(path))
	w.Header().Set("Accept-Ranges", "bytes")

	if fn, err := tryGetFileName(path); err == nil {
		setContentDisposition(w, true, fn)
	}

	if useRange && header != nil && header.Get("Range") != "" {
		ranges := parseRange(header.Get("Range"), size)
		if ranges == nil {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return nil
		}
		if len(ranges) == 1 {
			start, end := ranges[0][0], ranges[0][1]
			if _, err := f.Seek(int64(start), io.SeekStart); err != nil {
				return err
			}
			rangeSize := end - start + 1
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
			w.Header().Set("Content-Length", strconv.FormatUint(rangeSize, 10))
			w.WriteHeader(http.StatusPartialContent)
			if headOnly {
				return nil
			}
			_, err = io.CopyN(w, f, int64(rangeSize))
			return err
		}
		// multipart/byteranges
		boundary := randomUUID()
		var body []byte
		contentType := getContentType(path)
		for _, r := range ranges {
			start, end := r[0], r[1]
			if _, err := f.Seek(int64(start), io.SeekStart); err != nil {
				return err
			}
			rangeSize := end - start + 1
			partHeader := fmt.Sprintf("--%s\r\nContent-Type: %s\r\nContent-Range: bytes %d-%d/%d\r\n\r\n", boundary, contentType, start, end, size)
			body = append(body, partHeader...)
			buf := make([]byte, rangeSize)
			if _, err := io.ReadFull(f, buf); err != nil {
				return err
			}
			body = append(body, buf...)
			body = append(body, "\r\n"...)
		}
		body = append(body, fmt.Sprintf("--%s--\r\n", boundary)...)
		w.Header().Set("Content-Type", fmt.Sprintf("multipart/byteranges; boundary=%s", boundary))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusPartialContent)
		if headOnly {
			return nil
		}
		_, err = w.Write(body)
		return err
	}

	w.Header().Set("Content-Length", strconv.FormatUint(size, 10))
	w.WriteHeader(http.StatusOK)
	if headOnly {
		return nil
	}
	_, err = io.Copy(w, f)
	return err
}

func etagMatches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" {
			return true
		}
		if part == etag {
			return true
		}
		// weak comparison: W/"x" matches "x"
		if strings.HasPrefix(part, "W/") {
			if strings.TrimPrefix(part, "W/") == strings.TrimPrefix(etag, "W/") {
				return true
			}
		}
	}
	return false
}

// ---- send index ----

func (s *Server) sendIndex(path string, paths []PathItem, exist bool, q url.Values, headOnly bool, user string, accessPaths *AccessPaths, w *responseWriter) error {
	if sortKey := q.Get("sort"); sortKey != "" {
		switch sortKey {
		case "name":
			sortPaths(paths, compareName)
		case "mtime":
			sortPaths(paths, compareMtime)
		case "size":
			sortPaths(paths, compareSize)
		}
		if q.Get("order") == "desc" {
			reversePaths(paths)
		}
	} else {
		sortPaths(paths, compareName)
	}

	if hasQueryFlag(q, "simple") {
		var sb strings.Builder
		for i := range paths {
			name := escapeXML(paths[i].Name)
			sb.WriteString(name)
			if paths[i].isDir() {
				sb.WriteString("/\n")
			} else {
				sb.WriteString("\n")
			}
		}
		output := sb.String()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(output)))
		if headOnly {
			return nil
		}
		_, err := io.WriteString(w, output)
		return err
	}

	rel, err := filepath.Rel(s.args.servePath, path)
	if err != nil {
		return err
	}
	href := "/" + filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		href = "/"
	}
	readwrite := accessPaths.perm.readWrite()
	if paths == nil {
		paths = []PathItem{}
	}
	data := IndexData{
		Href:         href,
		Kind:         kindIndex,
		URIPrefix:    s.args.uriPrefix,
		AllowUpload:  s.args.allowUpload && readwrite,
		AllowDelete:  s.args.allowDelete && readwrite,
		AllowSearch:  s.args.allowSearch,
		AllowArchive: s.args.allowArchive,
		DirExists:    exist,
		Auth:         s.args.auth.hasUsers(),
		User:         strPtr(user),
		Paths:        paths,
	}

	var output string
	switch {
	case hasQueryFlag(q, "json"):
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		if headOnly {
			return nil
		}
		_, err = w.Write(b)
		return err
	case hasQueryFlag(q, "noscript"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		output = generateNoScriptHTML(&data)
	default:
		return s.renderApp(w, data, headOnly)
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(output)))
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if headOnly {
		return nil
	}
	_, err = io.WriteString(w, output)
	return err
}

func (s *Server) authReject(w *responseWriter) {
	setWebdavHeaders(w)
	if s.args.auth.useHashedPassword {
		w.Header().Set("WWW-Authenticate", `Basic realm="DUFS"`)
	} else {
		nonce := createNonce()
		w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Digest realm="DUFS", nonce="%s", qop="auth"`, nonce))
		w.Header().Add("WWW-Authenticate", `Basic realm="DUFS"`)
	}
	w.WriteHeader(http.StatusUnauthorized)
}

// ---- listing helpers ----

func (s *Server) listDir(entryPath, basePath string, accessPaths *AccessPaths) ([]PathItem, error) {
	var paths []PathItem
	if accessPaths.perm.indexOnly() {
		for _, name := range accessPaths.childNames() {
			s.addPathItem(&paths, basePath, filepath.Join(entryPath, name))
		}
	} else {
		entries, err := os.ReadDir(entryPath)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			s.addPathItem(&paths, basePath, filepath.Join(entryPath, e.Name()))
		}
	}
	return paths, nil
}

func (s *Server) addPathItem(paths *[]PathItem, basePath, entryPath string) {
	baseName := filepath.Base(entryPath)
	item, err := s.toPathItem(entryPath, basePath)
	if err != nil || item == nil {
		return
	}
	if isHidden(s.args.hidden, baseName, item.isDir()) {
		return
	}
	*paths = append(*paths, *item)
}

func (s *Server) toPathItem(p, basePath string) (*PathItem, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	linfo, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	isSymlink := linfo.Mode()&os.ModeSymlink != 0
	if !s.args.allowSymlink && isSymlink && !s.isRootContained(p) {
		return nil, nil
	}
	isDir := info.IsDir()
	var pathType PathType
	switch {
	case isSymlink && isDir:
		pathType = pathSymlinkDir
	case !isSymlink && isDir:
		pathType = pathDir
	case isSymlink && !isDir:
		pathType = pathSymlinkFile
	default:
		pathType = pathFile
	}
	mtime := toTimestamp(info.ModTime())
	var size uint64
	if pathType.isDir() {
		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		count := uint64(0)
		for _, e := range entries {
			if isHidden(s.args.hidden, e.Name(), e.IsDir()) {
				continue
			}
			count++
			if count >= maxSubpathsCount {
				break
			}
		}
		size = count
	} else {
		size = uint64(info.Size())
	}
	rel, err := filepath.Rel(basePath, p)
	if err != nil {
		return nil, err
	}
	return &PathItem{PathType: pathType, Name: filepath.ToSlash(rel), Mtime: mtime, Size: size}, nil
}

// collectDirEntries walks directories (following symlinks) and returns matching
// file paths.
func (s *Server) collectDirEntries(base string, accessPaths *AccessPaths, include func(p string, isFile bool) bool) []string {
	var paths []string
	for _, entry := range accessPaths.entryPaths(base) {
		visited := map[string]bool{}
		var rec func(p string, isRoot bool)
		rec = func(p string, isRoot bool) {
			if !s.running.Load() {
				return
			}
			info, err := os.Stat(p)
			if err != nil {
				return
			}
			isDir := info.IsDir()
			if !isRoot {
				baseName := filepath.Base(p)
				if isHidden(s.args.hidden, baseName, isDir) {
					return
				}
				if !s.args.allowSymlink {
					resolved, err := filepath.EvalSymlinks(p)
					if err != nil || !strings.HasPrefix(resolved, s.args.servePath) {
						return
					}
				}
			}
			if isDir {
				resolved := p
				if r, err := filepath.EvalSymlinks(p); err == nil {
					resolved = r
				}
				if visited[resolved] {
					return
				}
				visited[resolved] = true
				entries, err := os.ReadDir(p)
				if err != nil {
					return
				}
				for _, e := range entries {
					rec(filepath.Join(p, e.Name()), false)
				}
				return
			}
			if !include(p, true) {
				return
			}
			paths = append(paths, p)
		}
		rec(entry, true)
	}
	return paths
}

// ---- static helpers ----

func parseUploadOffset(header http.Header, size uint64) (*uint64, error) {
	v := header.Get("X-Update-Range")
	if v == "" {
		return nil, nil
	}
	if v == "append" {
		return &size, nil
	}
	ranges := parseRange(v, size)
	if ranges == nil {
		return nil, fmt.Errorf("invalid X-Update-Range header")
	}
	start := ranges[0][0]
	return &start, nil
}

func hasQueryFlag(q url.Values, name string) bool {
	vals, ok := q[name]
	if !ok {
		return false
	}
	return len(vals) > 0 && vals[0] == ""
}

func isHidden(hidden []string, fileName string, isDir bool) bool {
	for _, v := range hidden {
		if isDir {
			if x, ok := strings.CutSuffix(v, "/"); ok {
				if glob(x, fileName) {
					return true
				}
			}
		}
		if glob(v, fileName) {
			return true
		}
	}
	return false
}

func setWebdavHeaders(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET,HEAD,PUT,OPTIONS,DELETE,PATCH,PROPFIND,COPY,MOVE,CHECKAUTH,LOGOUT")
	w.Header().Set("DAV", "1, 2, 3")
}

func setContentDisposition(w http.ResponseWriter, inline bool, filename string) {
	kind := "inline"
	if !inline {
		kind = "attachment"
	}
	// strip control chars except tab
	var b strings.Builder
	for _, ch := range filename {
		if ch < 0x20 && ch != '\t' {
			b.WriteRune(' ')
		} else {
			b.WriteRune(ch)
		}
	}
	cleaned := b.String()
	var value string
	if isASCII(cleaned) {
		value = fmt.Sprintf(`%s; filename="%s"`, kind, cleaned)
	} else {
		value = fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, kind, cleaned, encodeURI(cleaned))
	}
	w.Header().Set("Content-Disposition", value)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func statusForbidden(w *responseWriter) {
	w.WriteHeader(http.StatusForbidden)
	io.WriteString(w, "Forbidden")
}

func statusNotFound(w *responseWriter) {
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, "Not Found")
}

func statusNoContent(w *responseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func statusBadRequest(w *responseWriter, body string) {
	w.WriteHeader(http.StatusBadRequest)
	if body != "" {
		io.WriteString(w, body)
	}
}

func resMultistatus(w *responseWriter, content string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	io.WriteString(w, "<?xml version=\"1.0\" encoding=\"utf-8\" ?>\n<D:multistatus xmlns:D=\"DAV:\">\n"+content+"</D:multistatus>")
}

func addCors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization,*")
	w.Header().Set("Access-Control-Expose-Headers", "Authorization,*")
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func ensurePathParent(path string) error {
	parent := filepath.Dir(path)
	if _, err := os.Lstat(parent); os.IsNotExist(err) {
		return os.MkdirAll(parent, 0o755)
	}
	return nil
}

func fileSize(path string) uint64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return uint64(info.Size())
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		now := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(now >> (8 * i))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ---- sorting ----

func sortPaths(paths []PathItem, cmp func(a, b *PathItem) int) {
	sort.SliceStable(paths, func(i, j int) bool {
		return cmp(&paths[i], &paths[j]) < 0
	})
}

func reversePaths(paths []PathItem) {
	for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
		paths[i], paths[j] = paths[j], paths[i]
	}
}

func typeOrder(t PathType) int {
	if t.isDir() {
		return 0
	}
	return 1
}

func compareName(a, b *PathItem) int {
	if c := typeOrder(a.PathType) - typeOrder(b.PathType); c != 0 {
		return c
	}
	return naturalCompare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

func compareMtime(a, b *PathItem) int {
	if c := typeOrder(a.PathType) - typeOrder(b.PathType); c != 0 {
		return c
	}
	switch {
	case a.Mtime < b.Mtime:
		return -1
	case a.Mtime > b.Mtime:
		return 1
	}
	return 0
}

func compareSize(a, b *PathItem) int {
	if c := typeOrder(a.PathType) - typeOrder(b.PathType); c != 0 {
		return c
	}
	switch {
	case a.Size < b.Size:
		return -1
	case a.Size > b.Size:
		return 1
	}
	return 0
}

func naturalCompare(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isDigit(ca) && isDigit(cb) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			ni, nj := si, sj
			for ni < i && a[ni] == '0' {
				ni++
			}
			for nj < j && b[nj] == '0' {
				nj++
			}
			if li, lj := i-ni, j-nj; li != lj {
				if li < lj {
					return -1
				}
				return 1
			}
			for ni < i && nj < j {
				if a[ni] != b[nj] {
					if a[ni] < b[nj] {
						return -1
					}
					return 1
				}
				ni++
				nj++
			}
		} else if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		} else {
			i++
			j++
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
