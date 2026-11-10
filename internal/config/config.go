package config

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/pflag"

	"github.com/zxysilent/gofs/internal/auth"
	"github.com/zxysilent/gofs/internal/fsutil"
	"github.com/zxysilent/gofs/internal/logx"
	"github.com/zxysilent/gofs/internal/urlutil"
)

// Compress maps to zip compression levels.
type Compress int

const (
	CompressNone Compress = iota
	CompressLow
	CompressMedium
	CompressHigh
)

func parseCompress(s string) (Compress, error) {
	switch strings.ToLower(s) {
	case "none":
		return CompressNone, nil
	case "", "low":
		return CompressLow, nil
	case "medium":
		return CompressMedium, nil
	case "high":
		return CompressHigh, nil
	}
	return CompressLow, fmt.Errorf("invalid compress level `%s`", s)
}

// BindAddr is either an IP address or a unix socket path.
type BindAddr struct {
	Addr string
	IsIP bool
}

// Args holds the validated, ready-to-use configuration. Everything derived
// (URIPrefix, PathIsFile, ErrorPage) is computed by options.resolve, so the
// server never has to re-derive anything.
type Args struct {
	ServePath      string
	Addrs          []BindAddr
	Port           int
	PathIsFile     bool
	PathPrefix     string
	URIPrefix      string
	Hidden         []string
	Auth           auth.AccessControl
	AllowAll       bool
	AllowUpload    bool
	AllowDelete    bool
	AllowSearch    bool
	AllowSymlink   bool
	AllowArchive   bool
	AllowHash      bool
	RenderIndex    bool
	RenderSpA      bool
	RenderTryIndex bool
	EnableCors     bool
	Assets         string             // "" means built-in assets
	ErrorPage      string             // "" means none
	AccessLog      *logx.AccessLogger // nil disables the access log
	LogFile        string
	LogLevel       string
	Compress       Compress
	TLSCert        string
	TLSKey         string
}

// DefaultArgs returns the configuration with every default applied.
func DefaultArgs() Args {
	// 默认值本身不可能非法，解析失败说明代码写错了。
	opts := defaultOptions()
	args, err := opts.resolve()
	if err != nil {
		panic(err)
	}
	return args
}

// isIPv6Available reports whether an IPv6 TCP socket can be created.
func isIPv6Available() bool {
	l, err := net.Listen("tcp6", "[::]:0")
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// ---- options: the single input shape ----
//
// options is the only thing TOML and the command line fill in. resolve turns it
// into Args, so validation lives in exactly one place instead of being spread
// over a "merge the file with the flags" routine.
//
// Every field is optional: a missing one keeps its defaultOptions value.

type options struct {
	ServePath      string   `toml:"serve-path"`
	Bind           []string `toml:"bind"`
	Port           int      `toml:"port"`
	PathPrefix     string   `toml:"path-prefix"`
	Hidden         []string `toml:"hidden"`
	Auth           []string `toml:"auth"`
	AllowAll       bool     `toml:"allow-all"`
	AllowUpload    bool     `toml:"allow-upload"`
	AllowDelete    bool     `toml:"allow-delete"`
	AllowSearch    bool     `toml:"allow-search"`
	AllowSymlink   bool     `toml:"allow-symlink"`
	AllowArchive   bool     `toml:"allow-archive"`
	AllowHash      bool     `toml:"allow-hash"`
	EnableCors     bool     `toml:"enable-cors"`
	RenderIndex    bool     `toml:"render-index"`
	RenderTryIndex bool     `toml:"render-try-index"`
	RenderSpA      bool     `toml:"render-spa"`
	Assets         string   `toml:"assets"`
	LogFile        string   `toml:"log-file"`
	LogLevel       string   `toml:"log-level"`
	Compress       string   `toml:"compress"`
	TLSCert        string   `toml:"tls-cert"`
	TLSKey         string   `toml:"tls-key"`
}

func defaultOptions() options {
	return options{
		ServePath: ".",
		Port:      5000,
		LogLevel:  "info",
		Compress:  "low",
	}
}

// resolve validates options and derives the final Args.
func (o *options) resolve() (Args, error) {
	servePath, err := sanitizePath(o.ServePath)
	if err != nil {
		return Args{}, err
	}
	if o.Port < 1 || o.Port > 65535 {
		return Args{}, fmt.Errorf("invalid port `%d`", o.Port)
	}
	if (o.TLSCert == "") != (o.TLSKey == "") {
		if o.TLSCert == "" {
			return Args{}, fmt.Errorf("no tls-cert set")
		}
		return Args{}, fmt.Errorf("no tls-key set")
	}
	compress, err := parseCompress(o.Compress)
	if err != nil {
		return Args{}, err
	}
	ac, err := auth.NewAccessControl(o.Auth)
	if err != nil {
		return Args{}, err
	}
	addrs, err := parseBindAddrs(o.Bind)
	if err != nil {
		return Args{}, err
	}

	pathPrefix := strings.Trim(o.PathPrefix, "/")
	uriPrefix := "/"
	if pathPrefix != "" {
		uriPrefix = "/" + urlutil.EncodeURI(pathPrefix) + "/"
	}

	args := Args{
		ServePath:      servePath,
		Addrs:          addrs,
		Port:           o.Port,
		PathIsFile:     fsutil.IsFile(servePath),
		PathPrefix:     pathPrefix,
		URIPrefix:      uriPrefix,
		Hidden:         o.Hidden,
		Auth:           ac,
		AllowAll:       o.AllowAll,
		AllowUpload:    o.AllowUpload,
		AllowDelete:    o.AllowDelete,
		AllowSearch:    o.AllowSearch,
		AllowSymlink:   o.AllowSymlink,
		AllowArchive:   o.AllowArchive,
		AllowHash:      o.AllowHash,
		RenderIndex:    o.RenderIndex,
		RenderSpA:      o.RenderSpA,
		RenderTryIndex: o.RenderTryIndex,
		EnableCors:     o.EnableCors,
		AccessLog:      &logx.AccessLogger{},
		LogFile:        o.LogFile,
		LogLevel:       o.LogLevel,
		Compress:       compress,
		TLSCert:        o.TLSCert,
		TLSKey:         o.TLSKey,
	}

	// --allow-all 是其余允许项的语法糖
	if args.AllowAll {
		args.AllowUpload = true
		args.AllowDelete = true
		args.AllowSearch = true
		args.AllowSymlink = true
		args.AllowArchive = true
		args.AllowHash = true
	}

	if o.Assets != "" {
		assets, err := sanitizeAssetsPath(o.Assets)
		if err != nil {
			return Args{}, err
		}
		args.Assets = assets
		if p := filepath.Join(assets, "404.html"); fsutil.IsFile(p) {
			args.ErrorPage = p
		}
	}
	return args, nil
}

// ---- flags ----

// meta collects what does not belong to Args: the help/version switches, and the
// destinations for the repeatable options. The lists land here rather than in
// options so pflag cannot append to values that came from the config file; a
// non-nil slice therefore means "this flag was given on the command line".
type meta struct {
	help        bool
	version     bool
	config      string
	completions string
	bind        []string
	hidden      []string
	auth        []string
}

// newFlagSet registers every option, bound onto opts. The values already in opts
// are the flag defaults, so anything a config file set survives unless the same
// flag appears on the command line.
//
// This set is also the single source of the help table and the completion list.
// A back-quoted word in a usage string becomes that option's value placeholder
// in the table (pflag's own convention), which is why they look like `file`.
func newFlagSet(o *options, m *meta) *pflag.FlagSet {
	fs := pflag.NewFlagSet("gofs", pflag.ContinueOnError)
	fs.SortFlags = false // registration order is the display order

	fs.StringVarP(&m.config, "config", "c", "", "configuration `file` (TOML)")
	fs.StringSliceVarP(&m.bind, "bind", "b", nil, "bind `addrs`, IP or unix socket")
	fs.IntVarP(&o.Port, "port", "p", o.Port, "port to listen on")
	fs.StringVar(&o.PathPrefix, "path-prefix", o.PathPrefix, "path `prefix`")
	fs.StringSliceVar(&m.hidden, "hidden", nil, "hide `paths` from listings, e.g. tmp,*.log")
	// StringArray, not StringSlice: one auth rule already contains commas.
	fs.StringArrayVarP(&m.auth, "auth", "a", nil, "auth `rule`, e.g. user:pass@/dir1:rw,/dir2")
	fs.BoolVarP(&o.AllowAll, "allow-all", "A", o.AllowAll, "allow all operations")
	fs.BoolVar(&o.AllowUpload, "allow-upload", o.AllowUpload, "allow upload files/folders")
	fs.BoolVar(&o.AllowDelete, "allow-delete", o.AllowDelete, "allow delete files/folders")
	fs.BoolVar(&o.AllowSearch, "allow-search", o.AllowSearch, "allow search files/folders")
	fs.BoolVar(&o.AllowSymlink, "allow-symlink", o.AllowSymlink, "allow symlink outside root directory")
	fs.BoolVar(&o.AllowArchive, "allow-archive", o.AllowArchive, "allow download folders as archive")
	fs.BoolVar(&o.AllowHash, "allow-hash", o.AllowHash, "allow ?hash query for file sha256")
	fs.BoolVar(&o.EnableCors, "enable-cors", o.EnableCors, "enable CORS for every origin")
	fs.BoolVar(&o.RenderIndex, "render-index", o.RenderIndex, "serve index.html for a directory, 404 if absent")
	fs.BoolVar(&o.RenderTryIndex, "render-try-index", o.RenderTryIndex, "serve index.html, else list the directory")
	fs.BoolVar(&o.RenderSpA, "render-spa", o.RenderSpA, "serve as a single page application")
	fs.StringVar(&o.Assets, "assets", o.Assets, "override the built-in assets `dir`")
	fs.StringVar(&o.LogFile, "log-file", o.LogFile, "write logs to a `file` instead of stderr")
	fs.StringVar(&o.LogLevel, "log-level", o.LogLevel, "log `level`: debug, info, warn, error")
	fs.StringVar(&o.Compress, "compress", o.Compress, "zip `level`: none, low, medium, high")
	fs.StringVar(&m.completions, "completions", "", "print a completion script for `shell` (bash)")
	fs.StringVar(&o.TLSCert, "tls-cert", o.TLSCert, "`path` to the TLS certificate for HTTPS")
	fs.StringVar(&o.TLSKey, "tls-key", o.TLSKey, "`path` to the TLS private key for HTTPS")
	fs.BoolVarP(&m.help, "help", "h", false, "print help")
	fs.BoolVarP(&m.version, "version", "V", false, "print version")
	return fs
}

// ---- config file ----

// loadConfig decodes a TOML file on top of o, leaving anything it does not
// mention untouched.
func loadConfig(path string, o *options) error {
	decoder, err := toml.DecodeFile(path, o)
	if err != nil {
		return fmt.Errorf("failed to load config at %s: %w", path, err)
	}
	// 拼错的键名会被静默忽略，配置看着生效其实没有 —— 直接报错更安全。
	if undecoded := decoder.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return fmt.Errorf("config `%s`: unknown key(s): %s", path, strings.Join(keys, ", "))
	}
	return nil
}

// ---- entry point ----

// Result is what ParseArgs returns: either the parsed configuration or a request
// for help / version / shell completions.
type Result struct {
	Args        Args
	Help        bool
	Version     bool
	Completions string
}

// ParseArgs reads the configuration from a TOML file (--config) and/or the
// command line: the file provides the defaults, an explicit flag wins over it.
// There are no environment variables and no hidden merge step -- the whole
// precedence rule is the two passes below.
func ParseArgs(argv []string) (*Result, error) {
	// 第一遍只为找到 --config：其余选项的默认值来自配置文件，必须先读文件。
	var first meta
	if err := newFlagSet(&options{}, &first).Parse(argv); err != nil {
		return nil, err
	}
	switch {
	case first.help:
		return &Result{Help: true}, nil
	case first.version:
		return &Result{Version: true}, nil
	case first.completions != "":
		return &Result{Completions: first.completions}, nil
	}

	opts := defaultOptions()
	if first.config != "" {
		if err := loadConfig(first.config, &opts); err != nil {
			return nil, err
		}
	}

	// 第二遍才真正生效：未出现在命令行的选项保留配置文件（或默认值）里的值。
	m := &meta{}
	fs := newFlagSet(&opts, m)
	if err := fs.Parse(argv); err != nil {
		return nil, err
	}
	if m.bind != nil {
		opts.Bind = m.bind
	}
	if m.hidden != nil {
		opts.Hidden = m.hidden
	}
	if m.auth != nil {
		opts.Auth = m.auth
	}
	if positional := fs.Args(); len(positional) > 0 {
		opts.ServePath = positional[0]
	}

	args, err := opts.resolve()
	if err != nil {
		return nil, err
	}
	return &Result{Args: args}, nil
}

// ---- validation helpers ----

func parseBindAddrs(addrs []string) ([]BindAddr, error) {
	if len(addrs) == 0 {
		out := []BindAddr{{Addr: "0.0.0.0", IsIP: true}}
		if isIPv6Available() {
			out = append(out, BindAddr{Addr: "::", IsIP: true})
		}
		return out, nil
	}
	var out []BindAddr
	for _, a := range addrs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if ip := net.ParseIP(a); ip != nil {
			out = append(out, BindAddr{Addr: a, IsIP: true})
		} else {
			// unix socket path
			out = append(out, BindAddr{Addr: a, IsIP: false})
		}
	}
	return out, nil
}

func sanitizePath(p string) (string, error) {
	if !fsutil.IsFile(p) && !fsutil.IsDir(p) {
		return "", fmt.Errorf("path `%s` doesn't exist", p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("failed to access path `%s`: %w", p, err)
	}
	canon, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("failed to access path `%s`: %w", p, err)
	}
	return canon, nil
}

func sanitizeAssetsPath(p string) (string, error) {
	canon, err := sanitizePath(p)
	if err != nil {
		return "", err
	}
	if !fsutil.IsFile(filepath.Join(canon, "index.html")) {
		return "", fmt.Errorf("path `%s` doesn't contains index.html", p)
	}
	return canon, nil
}
