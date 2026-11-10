package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

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

type cliMeta struct {
	help        bool
	version     bool
	completions string
}

// Result is what ParseArgs returns: either a parsed configuration or a request
// for help / version / shell completions.
type Result struct {
	Args        Args
	Help        bool
	Version     bool
	Completions string
}

var valueOpts = map[string]string{ // long -> canonical
	"--config":      "config",
	"--bind":        "bind",
	"--port":        "port",
	"--path-prefix": "path-prefix",
	"--hidden":      "hidden",
	"--auth":        "auth",
	"--assets":      "assets",
	"--log-file":    "log-file",
	"--log-level":   "log-level",
	"--compress":    "compress",
	"--tls-cert":    "tls-cert",
	"--tls-key":     "tls-key",
	"--completions": "completions",
}

var valueShortOpts = map[string]string{ // short -> canonical
	"-c": "config",
	"-b": "bind",
	"-p": "port",
	"-a": "auth",
}

var boolOpts = map[string]string{ // long -> canonical
	"--allow-all":        "allow-all",
	"--allow-upload":     "allow-upload",
	"--allow-delete":     "allow-delete",
	"--allow-search":     "allow-search",
	"--allow-symlink":    "allow-symlink",
	"--allow-archive":    "allow-archive",
	"--allow-hash":       "allow-hash",
	"--enable-cors":      "enable-cors",
	"--render-index":     "render-index",
	"--render-try-index": "render-try-index",
	"--render-spa":       "render-spa",
}

var boolShortOpts = map[string]string{
	"-A": "allow-all",
}

func parseCLI(argv []string) (cli map[string][]string, positional []string, meta cliMeta, err error) {
	cli = map[string][]string{}
	i := 0
	for i < len(argv) {
		arg := argv[i]
		switch {
		case arg == "--":
			positional = append(positional, argv[i+1:]...)
			return cli, positional, meta, nil
		case strings.HasPrefix(arg, "--"):
			name := arg
			val := ""
			hasEq := false
			if idx := strings.IndexByte(arg, '='); idx >= 0 {
				name = arg[:idx]
				val = arg[idx+1:]
				hasEq = true
			}
			switch name {
			case "--help":
				meta.help = true
				return cli, positional, meta, nil
			case "--version":
				meta.version = true
				return cli, positional, meta, nil
			}
			if canon, ok := valueOpts[name]; ok {
				if !hasEq {
					i++
					if i >= len(argv) {
						return nil, nil, meta, fmt.Errorf("missing value for %s", name)
					}
					val = argv[i]
				}
				cli[canon] = append(cli[canon], val)
			} else if canon, ok := boolOpts[name]; ok {
				cli[canon] = append(cli[canon], "")
			} else {
				return nil, nil, meta, fmt.Errorf("unknown flag `%s`", name)
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			first := arg[:2]
			if canon, ok := valueShortOpts[first]; ok {
				rest := strings.TrimPrefix(arg[2:], "=")
				if rest == "" {
					i++
					if i >= len(argv) {
						return nil, nil, meta, fmt.Errorf("missing value for %s", first)
					}
					rest = argv[i]
				}
				cli[canon] = append(cli[canon], rest)
			} else if canon, ok := boolShortOpts[first]; ok {
				cli[canon] = append(cli[canon], "")
			} else if first == "-h" {
				meta.help = true
				return cli, positional, meta, nil
			} else if first == "-V" {
				meta.version = true
				return cli, positional, meta, nil
			} else {
				return nil, nil, meta, fmt.Errorf("unknown flag `%s`", first)
			}
		default:
			positional = append(positional, arg)
		}
		i++
	}
	return cli, positional, meta, nil
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
	cli, positional, meta, err := parseCLI(argv)
	if err != nil {
		return nil, err
	}
	if meta.help || meta.version {
		return &Result{Help: meta.help, Version: meta.version}, nil
	}
	if v := cliValue(cli, "completions"); v != "" {
		return &Result{Completions: v}, nil
	}

	args := DefaultArgs()

	// config file
	if configPath := firstNonEmpty(cliValue(cli, "config"), os.Getenv("GOFS_CONFIG")); configPath != "" {
		args, err = loadConfig(configPath)
		if err != nil {
			return nil, err
		}
	}

	// serve path
	servePath := ""
	if len(positional) > 0 {
		servePath = positional[0]
	} else if v := os.Getenv("GOFS_SERVE_PATH"); v != "" {
		servePath = v
	} else {
		servePath = args.ServePath
	}
	sanitized, err := sanitizePath(servePath)
	if err != nil {
		return nil, err
	}
	args.ServePath = sanitized

	// port
	if v := firstNonEmpty(cliValue(cli, "port"), os.Getenv("GOFS_PORT")); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			args.Port = p
		} else {
			return nil, fmt.Errorf("invalid port `%s`", v)
		}
	}

	// bind addrs
	if v := multiValue(cli, "bind", true); len(v) > 0 {
		args.Addrs, err = parseBindAddrs(v)
		if err != nil {
			return nil, err
		}
	} else if v := os.Getenv("GOFS_BIND"); v != "" {
		args.Addrs, err = parseBindAddrs(strings.Split(v, ","))
		if err != nil {
			return nil, err
		}
	}

	args.PathIsFile = fsutil.IsFile(args.ServePath)

	// path prefix
	if v := firstNonEmpty(cliValue(cli, "path-prefix"), os.Getenv("GOFS_PATH_PREFIX")); v != "" {
		args.PathPrefix = v
	}
	args.PathPrefix = strings.Trim(args.PathPrefix, "/")
	if args.PathPrefix == "" {
		args.URIPrefix = "/"
	} else {
		args.URIPrefix = "/" + urlutil.EncodeURI(args.PathPrefix) + "/"
	}

	// hidden
	if v := multiValue(cli, "hidden", true); len(v) > 0 {
		args.Hidden = v
	} else if v := os.Getenv("GOFS_HIDDEN"); v != "" {
		args.Hidden = strings.Split(v, ",")
	}

	// auth
	if v := multiValue(cli, "auth", false); len(v) > 0 {
		args.Auth = mustAccessControl(v)
	} else if v := os.Getenv("GOFS_AUTH"); v != "" {
		args.Auth = mustAccessControl(splitAuthEnv(v))
	}

	// bool flags
	applyBool := func(canon, env string, target *bool) {
		if cliSet(cli, canon) {
			*target = true
		} else if v := os.Getenv(env); v != "" {
			*target = v != "false" && v != "0"
		}
	}
	applyBool("allow-all", "GOFS_ALLOW_ALL", &args.AllowAll)
	applyBool("allow-upload", "GOFS_ALLOW_UPLOAD", &args.AllowUpload)
	applyBool("allow-delete", "GOFS_ALLOW_DELETE", &args.AllowDelete)
	applyBool("allow-search", "GOFS_ALLOW_SEARCH", &args.AllowSearch)
	applyBool("allow-symlink", "GOFS_ALLOW_SYMLINK", &args.AllowSymlink)
	applyBool("allow-archive", "GOFS_ALLOW_ARCHIVE", &args.AllowArchive)
	applyBool("allow-hash", "GOFS_ALLOW_HASH", &args.AllowHash)
	applyBool("enable-cors", "GOFS_ENABLE_CORS", &args.EnableCors)
	applyBool("render-index", "GOFS_RENDER_INDEX", &args.RenderIndex)
	applyBool("render-try-index", "GOFS_RENDER_TRY_INDEX", &args.RenderTryIndex)
	applyBool("render-spa", "GOFS_RENDER_SPA", &args.RenderSpA)

	// allow-all implies all operation permissions
	allowAll := args.AllowAll
	if allowAll {
		args.AllowUpload = true
		args.AllowDelete = true
		args.AllowSearch = true
		args.AllowSymlink = true
		args.AllowArchive = true
		args.AllowHash = true
	}

	// assets
	if v := firstNonEmpty(cliValue(cli, "assets"), os.Getenv("GOFS_ASSETS")); v != "" {
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

	// log file
	if v := firstNonEmpty(cliValue(cli, "log-file"), os.Getenv("GOFS_LOG_FILE")); v != "" {
		args.LogFile = v
	}

	// log level
	if v := firstNonEmpty(cliValue(cli, "log-level"), os.Getenv("GOFS_LOG_LEVEL")); v != "" {
		args.LogLevel = v
	}

	// compress
	if v := firstNonEmpty(cliValue(cli, "compress"), os.Getenv("GOFS_COMPRESS")); v != "" {
		c, err := parseCompress(v)
		if err != nil {
			return nil, err
		}
		args.Compress = c
	}

	// tls
	if v := firstNonEmpty(cliValue(cli, "tls-cert"), os.Getenv("GOFS_TLS_CERT")); v != "" {
		args.TLSCert = v
	}
	if v := firstNonEmpty(cliValue(cli, "tls-key"), os.Getenv("GOFS_TLS_KEY")); v != "" {
		args.TLSKey = v
	}
	if (args.TLSCert == "") != (args.TLSKey == "") {
		if args.TLSCert == "" {
			return nil, fmt.Errorf("no tls-cert set")
		}
		return nil, fmt.Errorf("no tls-key set")
	}

	return &Result{Args: args}, nil
}

func cliValue(cli map[string][]string, canon string) string {
	if v, ok := cli[canon]; ok && len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}

func cliSet(cli map[string][]string, canon string) bool {
	_, ok := cli[canon]
	return ok
}

func multiValue(cli map[string][]string, canon string, commaSplit bool) []string {
	v := cli[canon]
	if len(v) == 0 {
		return nil
	}
	if !commaSplit {
		return v
	}
	var out []string
	for _, s := range v {
		for _, part := range strings.Split(s, ",") {
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
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
