package config

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
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

// Args holds all configuration.
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
	AccessLog      *logx.AccessLogger // nil 表示关闭访问日志
	LogFile        string
	LogLevel       string // "" means info
	Compress       Compress
	TLSCert        string
	TLSKey         string
}

func DefaultArgs() Args {
	addrs := []BindAddr{{Addr: "0.0.0.0", IsIP: true}}
	if isIPv6Available() {
		addrs = append(addrs, BindAddr{Addr: "::", IsIP: true})
	}
	return Args{
		ServePath: ".",
		Addrs:     addrs,
		Port:      5000,
		Auth:      mustAccessControl(nil),
		AccessLog: &logx.AccessLogger{},
		Compress:  CompressLow,
	}
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

func mustAccessControl(rules []string) auth.AccessControl {
	ac, err := auth.NewAccessControl(rules)
	if err != nil {
		panic(err)
	}
	return ac
}

// ---- CLI parsing ----

// Result is what ParseArgs returns: either a parsed configuration or a request
// for help / version / shell completions.
type Result struct {
	Args        Args
	Help        bool
	Version     bool
	Completions string
}

// ---- CLI parsing ----
//
// pflag 提供 POSIX/GNU 风格：-p 5000、--port=5000、-p5000、-Ab（短选项合并）
// 都支持，并且默认允许位置参数与选项交错出现（gofs /srv/data -A）。
// flag 包这三样都做不到，所以不用它。

// flags 收集命令行上出现的值。只有 fs.Changed(name) 为真时，对应字段才有意义；
// 否则该选项应当回退到环境变量、配置文件或默认值。
type flags struct {
	config         string
	bind           []string
	port           int
	pathPrefix     string
	hidden         []string
	auth           []string
	assets         string
	logFile        string
	logLevel       string
	compress       string
	tlsCert        string
	tlsKey         string
	completions    string
	allowAll       bool
	allowUpload    bool
	allowDelete    bool
	allowSearch    bool
	allowSymlink   bool
	allowArchive   bool
	allowHash      bool
	enableCors     bool
	renderIndex    bool
	renderTryIndex bool
	renderSpA      bool
	help           bool
	version        bool
}

// newFlagSet registers every option. This is the single source of truth for
// the CLI: help (PrintHelp) and shell completions (PrintCompletions) are both
// generated from it, so adding a flag cannot leave either out of date.
func newFlagSet(f *flags) *pflag.FlagSet {
	fs := pflag.NewFlagSet("gofs", pflag.ContinueOnError)
	fs.SortFlags = false // keep registration order, which is the display order
	fs.Usage = func() {}
	fs.SetOutput(io.Discard)

	strVarP(fs, &f.config, "config", "c", "<file>", "configuration file (TOML)")
	sliceVarP(fs, &f.bind, "bind", "b", "<addrs>", "bind address or unix socket")
	intVarP(fs, &f.port, "port", "p", "<port>", "port to listen on (default 5000)")
	strVar(fs, &f.pathPrefix, "path-prefix", "<path>", "path prefix")
	sliceVar(fs, &f.hidden, "hidden", "<value>", "hide paths from listings, e.g. tmp,*.log")
	arrayVarP(fs, &f.auth, "auth", "a", "<rules>", "auth rules, e.g. user:pass@/dir1:rw,/dir2")
	fs.BoolVarP(&f.allowAll, "allow-all", "A", false, "allow all operations")
	fs.BoolVar(&f.allowUpload, "allow-upload", false, "allow upload files/folders")
	fs.BoolVar(&f.allowDelete, "allow-delete", false, "allow delete files/folders")
	fs.BoolVar(&f.allowSearch, "allow-search", false, "allow search files/folders")
	fs.BoolVar(&f.allowSymlink, "allow-symlink", false, "allow symlink outside root directory")
	fs.BoolVar(&f.allowArchive, "allow-archive", false, "allow download folders as archive")
	fs.BoolVar(&f.allowHash, "allow-hash", false, "allow ?hash query for file sha256")
	fs.BoolVar(&f.enableCors, "enable-cors", false, "enable CORS for every origin")
	fs.BoolVar(&f.renderIndex, "render-index", false, "serve index.html for a directory, 404 if absent")
	fs.BoolVar(&f.renderTryIndex, "render-try-index", false, "serve index.html, else list the directory")
	fs.BoolVar(&f.renderSpA, "render-spa", false, "serve as a single page application")
	strVar(fs, &f.assets, "assets", "<path>", "override the built-in assets directory")
	strVar(fs, &f.logFile, "log-file", "<file>", "write logs to a file instead of stderr")
	strVar(fs, &f.logLevel, "log-level", "<level>", "log level: debug, info, warn, error (default info)")
	strVar(fs, &f.compress, "compress", "<level>", "zip level: none, low, medium, high (default low)")
	strVar(fs, &f.completions, "completions", "<shell>", "print a shell completion script (bash)")
	strVar(fs, &f.tlsCert, "tls-cert", "<path>", "TLS certificate for HTTPS")
	strVar(fs, &f.tlsKey, "tls-key", "<path>", "TLS private key for HTTPS")
	fs.BoolVarP(&f.help, "help", "h", false, "print help")
	fs.BoolVarP(&f.version, "version", "V", false, "print version")
	return fs
}

// shownType overrides the type column of pflag's generated usage table, so it
// reads "<file>" instead of "string".
type shownType struct {
	pflag.Value
	name string
}

func (v shownType) Type() string { return v.name }

func labelType(fs *pflag.FlagSet, name, shown string) {
	fl := fs.Lookup(name)
	fl.Value = shownType{fl.Value, shown}
	// 包装后 pflag 的类型断言（*stringSliceValue 等）失效，"[]" 不再被认作
	// 零值，帮助里会多出 "(default [])"。空字符串是它 default 分支认可的零值。
	if fl.DefValue == "[]" {
		fl.DefValue = ""
	}
}

func strVar(fs *pflag.FlagSet, p *string, name, shown, usage string) {
	fs.StringVar(p, name, "", usage)
	labelType(fs, name, shown)
}

func strVarP(fs *pflag.FlagSet, p *string, name, short, shown, usage string) {
	fs.StringVarP(p, name, short, "", usage)
	labelType(fs, name, shown)
}

func intVarP(fs *pflag.FlagSet, p *int, name, short, shown, usage string) {
	fs.IntVarP(p, name, short, 0, usage)
	labelType(fs, name, shown)
}

// sliceVar is a comma-separated list (--hidden a,b).
func sliceVar(fs *pflag.FlagSet, p *[]string, name, shown, usage string) {
	fs.StringSliceVar(p, name, nil, usage)
	labelType(fs, name, shown)
}

func sliceVarP(fs *pflag.FlagSet, p *[]string, name, short, shown, usage string) {
	fs.StringSliceVarP(p, name, short, nil, usage)
	labelType(fs, name, shown)
}

// arrayVar keeps every repetition separate (--auth "...,:rw" --auth "..."),
// because auth rules contain commas themselves.
func arrayVarP(fs *pflag.FlagSet, p *[]string, name, short, shown, usage string) {
	fs.StringArrayVarP(p, name, short, nil, usage)
	labelType(fs, name, shown)
}

func parseCLI(argv []string) (*flags, *pflag.FlagSet, []string, error) {
	f := &flags{}
	fs := newFlagSet(f)
	if err := fs.Parse(argv); err != nil {
		return nil, nil, nil, err
	}
	return f, fs, fs.Args(), nil
}

// pickString resolves one string option: an explicit command-line flag wins,
// then the GOFS_* environment variable, then cur (the config-file value, which
// already fell back to the default).
func pickString(fs *pflag.FlagSet, name, value, env, cur string) string {
	if fs.Changed(name) {
		return value
	}
	if v := os.Getenv(env); v != "" {
		return v
	}
	return cur
}

// pickBool is the boolean counterpart of pickString. GOFS_*="false"/"0" turns
// a boolean off explicitly, which is the only way to do so from the environment.
func pickBool(fs *pflag.FlagSet, name string, value bool, env string, cur bool) bool {
	if fs.Changed(name) {
		return value
	}
	if v := os.Getenv(env); v != "" {
		return v != "false" && v != "0"
	}
	return cur
}

// splitCommaList splits an environment value such as "tmp,*.log".
func splitCommaList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// fileConfig is the TOML shape of the config file. Every field is optional:
// anything that is absent keeps the value from DefaultArgs(), and the CLI and
// GOFS_* environment variables still win over the file.
//
// Keys use the same dash-separated names as the CLI flags.
type fileConfig struct {
	ServePath      string     `toml:"serve-path"`
	Bind           stringList `toml:"bind"`
	Port           int        `toml:"port"`
	PathPrefix     string     `toml:"path-prefix"`
	Hidden         stringList `toml:"hidden"`
	Auth           stringList `toml:"auth"`
	AllowAll       bool       `toml:"allow-all"`
	AllowUpload    bool       `toml:"allow-upload"`
	AllowDelete    bool       `toml:"allow-delete"`
	AllowSearch    bool       `toml:"allow-search"`
	AllowSymlink   bool       `toml:"allow-symlink"`
	AllowArchive   bool       `toml:"allow-archive"`
	AllowHash      bool       `toml:"allow-hash"`
	EnableCors     bool       `toml:"enable-cors"`
	RenderIndex    bool       `toml:"render-index"`
	RenderTryIndex bool       `toml:"render-try-index"`
	RenderSpA      bool       `toml:"render-spa"`
	Assets         string     `toml:"assets"`
	LogFile        string     `toml:"log-file"`
	LogLevel       string     `toml:"log-level"`
	Compress       string     `toml:"compress"`
	TLSCert        string     `toml:"tls-cert"`
	TLSKey         string     `toml:"tls-key"`
}

// stringList accepts either a single string or an array of strings, so both
// `bind = "127.0.0.1"` and `bind = ["127.0.0.1", "::"]` work.
type stringList []string

func (s *stringList) UnmarshalTOML(v interface{}) error {
	switch t := v.(type) {
	case string:
		*s = stringList{t}
	case []interface{}:
		out := make(stringList, 0, len(t))
		for _, item := range t {
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("expected a string, got %T", item)
			}
			out = append(out, str)
		}
		*s = out
	default:
		return fmt.Errorf("expected a string or an array of strings, got %T", v)
	}
	return nil
}

func loadConfig(path string) (Args, error) {
	// 旧版本读的是 YAML；直接指出格式已变，比抛一个 TOML 语法错误有用得多。
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return Args{}, fmt.Errorf("config `%s`: YAML is no longer supported, use TOML", path)
	}

	var cfg fileConfig
	meta, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Args{}, fmt.Errorf("failed to load config at %s: %w", path, err)
	}
	// 拼错的键名会被静默忽略，配置看着生效其实没有 —— 直接报错更安全。
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return Args{}, fmt.Errorf("config `%s`: unknown key(s): %s", path, strings.Join(keys, ", "))
	}

	args := DefaultArgs()
	if cfg.ServePath != "" {
		args.ServePath = cfg.ServePath
	}
	if len(cfg.Bind) > 0 {
		addrs, err := parseBindAddrs(cfg.Bind)
		if err != nil {
			return Args{}, fmt.Errorf("config `%s`: %w", path, err)
		}
		args.Addrs = addrs
	}
	if cfg.Port != 0 {
		if cfg.Port < 0 || cfg.Port > 65535 {
			return Args{}, fmt.Errorf("config `%s`: invalid port %d", path, cfg.Port)
		}
		args.Port = cfg.Port
	}
	if cfg.PathPrefix != "" {
		args.PathPrefix = cfg.PathPrefix
	}
	if len(cfg.Hidden) > 0 {
		args.Hidden = cfg.Hidden
	}
	if len(cfg.Auth) > 0 {
		// 用返回错误而不是 panic 的版本，配置写错时给出可读提示。
		ac, err := auth.NewAccessControl(cfg.Auth)
		if err != nil {
			return Args{}, fmt.Errorf("config `%s`: %w", path, err)
		}
		args.Auth = ac
	}
	args.AllowAll = cfg.AllowAll
	args.AllowUpload = cfg.AllowUpload
	args.AllowDelete = cfg.AllowDelete
	args.AllowSearch = cfg.AllowSearch
	args.AllowSymlink = cfg.AllowSymlink
	args.AllowArchive = cfg.AllowArchive
	args.AllowHash = cfg.AllowHash
	args.EnableCors = cfg.EnableCors
	args.RenderIndex = cfg.RenderIndex
	args.RenderTryIndex = cfg.RenderTryIndex
	args.RenderSpA = cfg.RenderSpA
	args.Assets = cfg.Assets
	args.LogFile = cfg.LogFile
	args.LogLevel = cfg.LogLevel
	if cfg.Compress != "" {
		c, err := parseCompress(cfg.Compress)
		if err != nil {
			return Args{}, fmt.Errorf("config `%s`: %w", path, err)
		}
		args.Compress = c
	}
	args.TLSCert = cfg.TLSCert
	args.TLSKey = cfg.TLSKey
	return args, nil
}

// ---- argument assembly ----

func ParseArgs(argv []string) (*Result, error) {
	f, fs, positional, err := parseCLI(argv)
	if err != nil {
		return nil, err
	}
	if f.help {
		return &Result{Help: true}, nil
	}
	if f.version {
		return &Result{Version: true}, nil
	}
	if f.completions != "" {
		return &Result{Completions: f.completions}, nil
	}

	args := DefaultArgs()

	// config file: 作为其余选项的基线，随后被环境变量与命令行覆盖
	if configPath := pickString(fs, "config", f.config, "GOFS_CONFIG", ""); configPath != "" {
		args, err = loadConfig(configPath)
		if err != nil {
			return nil, err
		}
	}

	// serve path：位置参数 > GOFS_SERVE_PATH > 配置文件
	servePath := args.ServePath
	if len(positional) > 0 {
		servePath = positional[0]
	} else if v := os.Getenv("GOFS_SERVE_PATH"); v != "" {
		servePath = v
	}
	sanitized, err := sanitizePath(servePath)
	if err != nil {
		return nil, err
	}
	args.ServePath = sanitized

	// port
	if fs.Changed("port") {
		if f.port < 0 || f.port > 65535 {
			return nil, fmt.Errorf("invalid port `%d`", f.port)
		}
		args.Port = f.port
	} else if v := os.Getenv("GOFS_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid port `%s`", v)
		}
		args.Port = p
	}

	// bind addrs
	if fs.Changed("bind") {
		args.Addrs, err = parseBindAddrs(f.bind)
		if err != nil {
			return nil, err
		}
	} else if v := os.Getenv("GOFS_BIND"); v != "" {
		args.Addrs, err = parseBindAddrs(splitCommaList(v))
		if err != nil {
			return nil, err
		}
	}

	args.PathIsFile = fsutil.IsFile(args.ServePath)

	// path prefix
	args.PathPrefix = pickString(fs, "path-prefix", f.pathPrefix, "GOFS_PATH_PREFIX", args.PathPrefix)
	args.PathPrefix = strings.Trim(args.PathPrefix, "/")
	if args.PathPrefix == "" {
		args.URIPrefix = "/"
	} else {
		args.URIPrefix = "/" + urlutil.EncodeURI(args.PathPrefix) + "/"
	}

	// hidden
	if fs.Changed("hidden") {
		args.Hidden = f.hidden
	} else if v := os.Getenv("GOFS_HIDDEN"); v != "" {
		args.Hidden = splitCommaList(v)
	}

	// auth：规则写错时给出错误而不是 panic（命令行同样来自用户输入）
	if fs.Changed("auth") {
		args.Auth, err = auth.NewAccessControl(f.auth)
		if err != nil {
			return nil, err
		}
	} else if v := os.Getenv("GOFS_AUTH"); v != "" {
		args.Auth, err = auth.NewAccessControl(splitAuthEnv(v))
		if err != nil {
			return nil, err
		}
	}

	// bool flags
	args.AllowAll = pickBool(fs, "allow-all", f.allowAll, "GOFS_ALLOW_ALL", args.AllowAll)
	args.AllowUpload = pickBool(fs, "allow-upload", f.allowUpload, "GOFS_ALLOW_UPLOAD", args.AllowUpload)
	args.AllowDelete = pickBool(fs, "allow-delete", f.allowDelete, "GOFS_ALLOW_DELETE", args.AllowDelete)
	args.AllowSearch = pickBool(fs, "allow-search", f.allowSearch, "GOFS_ALLOW_SEARCH", args.AllowSearch)
	args.AllowSymlink = pickBool(fs, "allow-symlink", f.allowSymlink, "GOFS_ALLOW_SYMLINK", args.AllowSymlink)
	args.AllowArchive = pickBool(fs, "allow-archive", f.allowArchive, "GOFS_ALLOW_ARCHIVE", args.AllowArchive)
	args.AllowHash = pickBool(fs, "allow-hash", f.allowHash, "GOFS_ALLOW_HASH", args.AllowHash)
	args.EnableCors = pickBool(fs, "enable-cors", f.enableCors, "GOFS_ENABLE_CORS", args.EnableCors)
	args.RenderIndex = pickBool(fs, "render-index", f.renderIndex, "GOFS_RENDER_INDEX", args.RenderIndex)
	args.RenderTryIndex = pickBool(fs, "render-try-index", f.renderTryIndex, "GOFS_RENDER_TRY_INDEX", args.RenderTryIndex)
	args.RenderSpA = pickBool(fs, "render-spa", f.renderSpA, "GOFS_RENDER_SPA", args.RenderSpA)

	// allow-all implies all operation permissions
	if args.AllowAll {
		args.AllowUpload = true
		args.AllowDelete = true
		args.AllowSearch = true
		args.AllowSymlink = true
		args.AllowArchive = true
		args.AllowHash = true
	}

	// assets
	if v := pickString(fs, "assets", f.assets, "GOFS_ASSETS", args.Assets); v != "" {
		sanitizedAssets, err := sanitizeAssetsPath(v)
		if err != nil {
			return nil, err
		}
		args.Assets = sanitizedAssets
	}
	if args.Assets != "" {
		if p := filepath.Join(args.Assets, "404.html"); fsutil.IsFile(p) {
			args.ErrorPage = p
		}
	}

	args.LogFile = pickString(fs, "log-file", f.logFile, "GOFS_LOG_FILE", args.LogFile)
	args.LogLevel = pickString(fs, "log-level", f.logLevel, "GOFS_LOG_LEVEL", args.LogLevel)

	// compress
	if v := pickString(fs, "compress", f.compress, "GOFS_COMPRESS", ""); v != "" {
		c, err := parseCompress(v)
		if err != nil {
			return nil, err
		}
		args.Compress = c
	}

	// tls
	args.TLSCert = pickString(fs, "tls-cert", f.tlsCert, "GOFS_TLS_CERT", args.TLSCert)
	args.TLSKey = pickString(fs, "tls-key", f.tlsKey, "GOFS_TLS_KEY", args.TLSKey)
	if (args.TLSCert == "") != (args.TLSKey == "") {
		if args.TLSCert == "" {
			return nil, fmt.Errorf("no tls-cert set")
		}
		return nil, fmt.Errorf("no tls-key set")
	}

	return &Result{Args: args}, nil
}

// splitAuthEnv splits GOFS_AUTH on '|' the same way multiple -a rules are.
func splitAuthEnv(v string) []string {
	return auth.SplitRules([]string{v})
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

func parseBindAddrs(addrs []string) ([]BindAddr, error) {
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
