package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Compress maps to zip compression levels.
type Compress int

const (
	compressNone Compress = iota
	compressLow
	compressMedium
	compressHigh
)

func parseCompress(s string) (Compress, error) {
	switch strings.ToLower(s) {
	case "none":
		return compressNone, nil
	case "", "low":
		return compressLow, nil
	case "medium":
		return compressMedium, nil
	case "high":
		return compressHigh, nil
	}
	return compressLow, fmt.Errorf("invalid compress level `%s`", s)
}

// BindAddr is either an IP address or a unix socket path.
type BindAddr struct {
	addr string
	isIP bool
}

// Args holds all configuration.
type Args struct {
	servePath      string
	addrs          []BindAddr
	port           int
	pathIsFile     bool
	pathPrefix     string
	uriPrefix      string
	hidden         []string
	auth           AccessControl
	allowAll       bool
	allowUpload    bool
	allowDelete    bool
	allowSearch    bool
	allowSymlink   bool
	allowArchive   bool
	allowHash      bool
	renderIndex    bool
	renderSpA      bool
	renderTryIndex bool
	enableCors     bool
	assets         string // "" means built-in assets
	errorPage      string // "" means none
	httpLogger     HttpLogger
	logFile        string
	compress       Compress
	tlsCert        string
	tlsKey         string
}

func defaultArgs() Args {
	addrs := []BindAddr{{addr: "0.0.0.0", isIP: true}}
	if isIPv6Available() {
		addrs = append(addrs, BindAddr{addr: "::", isIP: true})
	}
	return Args{
		servePath:  ".",
		addrs:      addrs,
		port:       5000,
		auth:       mustAccessControl(nil),
		httpLogger: defaultHttpLogger(),
		compress:   compressLow,
	}
}

func mustAccessControl(rules []string) AccessControl {
	ac, err := newAccessControl(rules)
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

type parseResult struct {
	args        Args
	help        bool
	version     bool
	completions string
}

var valueOpts = map[string]string{ // long -> canonical
	"--config":      "config",
	"--bind":        "bind",
	"--port":        "port",
	"--path-prefix": "path-prefix",
	"--hidden":      "hidden",
	"--auth":        "auth",
	"--assets":      "assets",
	"--log-format":  "log-format",
	"--log-file":    "log-file",
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

// ---- Config (YAML subset) parsing ----

func parseYAML(data string) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	lines := strings.Split(data, "\n")
	var currentKey string
	for i := 0; i < len(lines); i++ {
		line := stripYAMLComment(lines[i])
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "---" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			item = unquoteYAML(item)
			if currentKey != "" {
				if list, ok := result[currentKey].([]string); ok {
					result[currentKey] = append(list, item)
				} else {
					result[currentKey] = []string{item}
				}
			}
			continue
		}
		if idx := strings.IndexByte(trimmed, ':'); idx >= 0 {
			key := strings.TrimSpace(trimmed[:idx])
			val := strings.TrimSpace(trimmed[idx+1:])
			currentKey = key
			if val == "" {
				result[key] = []string{}
			} else {
				result[key] = parseYAMLScalar(val)
			}
		}
	}
	return result, nil
}

func parseYAMLScalar(v string) interface{} {
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		inner := strings.TrimSpace(v[1 : len(v)-1])
		if inner == "" {
			return []string{}
		}
		var items []string
		for _, it := range splitYAMLList(inner) {
			items = append(items, unquoteYAML(strings.TrimSpace(it)))
		}
		return items
	}
	unq := unquoteYAML(v)
	if b, err := strconv.ParseBool(unq); err == nil {
		return b
	}
	return unq
}

func splitYAMLList(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			cur.WriteByte(c)
			continue
		}
		if c == '\'' || c == '"' {
			inQuote = c
			cur.WriteByte(c)
			continue
		}
		if c == ',' {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func stripYAMLComment(s string) string {
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			inQuote = c
			continue
		}
		if c == '#' {
			return s[:i]
		}
	}
	return s
}

func unquoteYAML(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func loadConfig(path string) (Args, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Args{}, fmt.Errorf("failed to read config at %s: %w", path, err)
	}
	m, err := parseYAML(string(contents))
	if err != nil {
		return Args{}, fmt.Errorf("failed to load config at %s: %w", path, err)
	}
	args := defaultArgs()
	if v, ok := m["serve-path"].(string); ok {
		args.servePath = v
	}
	if v := yamlStringList(m["bind"]); v != nil {
		args.addrs, _ = parseBindAddrs(v)
	}
	if v, ok := m["port"].(string); ok {
		if p, err := strconv.Atoi(v); err == nil {
			args.port = p
		}
	}
	if v, ok := m["path-prefix"].(string); ok {
		args.pathPrefix = v
	}
	if v := yamlStringList(m["hidden"]); v != nil {
		args.hidden = v
	}
	if v := yamlStringList(m["auth"]); v != nil {
		args.auth = mustAccessControl(v)
	}
	args.allowAll = yamlBool(m["allow-all"])
	args.allowUpload = yamlBool(m["allow-upload"])
	args.allowDelete = yamlBool(m["allow-delete"])
	args.allowSearch = yamlBool(m["allow-search"])
	args.allowSymlink = yamlBool(m["allow-symlink"])
	args.allowArchive = yamlBool(m["allow-archive"])
	args.allowHash = yamlBool(m["allow-hash"])
	args.enableCors = yamlBool(m["enable-cors"])
	args.renderIndex = yamlBool(m["render-index"])
	args.renderTryIndex = yamlBool(m["render-try-index"])
	args.renderSpA = yamlBool(m["render-spa"])
	if v, ok := m["assets"].(string); ok {
		args.assets = v
	}
	if v, ok := m["log-format"].(string); ok {
		args.httpLogger = parseHttpLogger(v)
	}
	if v, ok := m["log-file"].(string); ok {
		args.logFile = v
	}
	if v, ok := m["compress"].(string); ok {
		if c, err := parseCompress(v); err == nil {
			args.compress = c
		}
	}
	if v, ok := m["tls-cert"].(string); ok {
		args.tlsCert = v
	}
	if v, ok := m["tls-key"].(string); ok {
		args.tlsKey = v
	}
	return args, nil
}

func yamlStringList(v interface{}) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []string:
		return t
	}
	return nil
}

func yamlBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

// ---- argument assembly ----

func parseArgs(argv []string) (*parseResult, error) {
	cli, positional, meta, err := parseCLI(argv)
	if err != nil {
		return nil, err
	}
	if meta.help || meta.version {
		return &parseResult{help: meta.help, version: meta.version}, nil
	}
	if v := cliValue(cli, "completions"); v != "" {
		return &parseResult{completions: v}, nil
	}

	args := defaultArgs()

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
		servePath = args.servePath
	}
	sanitized, err := sanitizePath(servePath)
	if err != nil {
		return nil, err
	}
	args.servePath = sanitized

	// port
	if v := firstNonEmpty(cliValue(cli, "port"), os.Getenv("GOFS_PORT")); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			args.port = p
		} else {
			return nil, fmt.Errorf("invalid port `%s`", v)
		}
	}

	// bind addrs
	if v := multiValue(cli, "bind", true); len(v) > 0 {
		args.addrs, err = parseBindAddrs(v)
		if err != nil {
			return nil, err
		}
	} else if v := os.Getenv("GOFS_BIND"); v != "" {
		args.addrs, err = parseBindAddrs(strings.Split(v, ","))
		if err != nil {
			return nil, err
		}
	}

	args.pathIsFile = isFile(args.servePath)

	// path prefix
	if v := firstNonEmpty(cliValue(cli, "path-prefix"), os.Getenv("GOFS_PATH_PREFIX")); v != "" {
		args.pathPrefix = v
	}
	args.pathPrefix = strings.Trim(args.pathPrefix, "/")
	if args.pathPrefix == "" {
		args.uriPrefix = "/"
	} else {
		args.uriPrefix = "/" + encodeURI(args.pathPrefix) + "/"
	}

	// hidden
	if v := multiValue(cli, "hidden", true); len(v) > 0 {
		args.hidden = v
	} else if v := os.Getenv("GOFS_HIDDEN"); v != "" {
		args.hidden = strings.Split(v, ",")
	}

	// auth
	if v := multiValue(cli, "auth", false); len(v) > 0 {
		args.auth = mustAccessControl(v)
	} else if v := os.Getenv("GOFS_AUTH"); v != "" {
		args.auth = mustAccessControl(splitAuthEnv(v))
	}

	// bool flags
	applyBool := func(canon, env string, target *bool) {
		if cliSet(cli, canon) {
			*target = true
		} else if v := os.Getenv(env); v != "" {
			*target = v != "false" && v != "0"
		}
	}
	applyBool("allow-all", "GOFS_ALLOW_ALL", &args.allowAll)
	applyBool("allow-upload", "GOFS_ALLOW_UPLOAD", &args.allowUpload)
	applyBool("allow-delete", "GOFS_ALLOW_DELETE", &args.allowDelete)
	applyBool("allow-search", "GOFS_ALLOW_SEARCH", &args.allowSearch)
	applyBool("allow-symlink", "GOFS_ALLOW_SYMLINK", &args.allowSymlink)
	applyBool("allow-archive", "GOFS_ALLOW_ARCHIVE", &args.allowArchive)
	applyBool("allow-hash", "GOFS_ALLOW_HASH", &args.allowHash)
	applyBool("enable-cors", "GOFS_ENABLE_CORS", &args.enableCors)
	applyBool("render-index", "GOFS_RENDER_INDEX", &args.renderIndex)
	applyBool("render-try-index", "GOFS_RENDER_TRY_INDEX", &args.renderTryIndex)
	applyBool("render-spa", "GOFS_RENDER_SPA", &args.renderSpA)

	// allow-all implies all operation permissions
	allowAll := args.allowAll
	if allowAll {
		args.allowUpload = true
		args.allowDelete = true
		args.allowSearch = true
		args.allowSymlink = true
		args.allowArchive = true
		args.allowHash = true
	}

	// assets
	if v := firstNonEmpty(cliValue(cli, "assets"), os.Getenv("GOFS_ASSETS")); v != "" {
		sanitizedAssets, err := sanitizeAssetsPath(v)
		if err != nil {
			return nil, err
		}
		args.assets = sanitizedAssets
	}
	if args.assets != "" {
		if p := filepath.Join(args.assets, "404.html"); fileExists(p) {
			args.errorPage = p
		}
	}

	// log format
	if v := firstNonEmpty(cliValue(cli, "log-format"), os.Getenv("GOFS_LOG_FORMAT")); v != "" {
		args.httpLogger = parseHttpLogger(v)
	} else if cliSet(cli, "log-format") {
		args.httpLogger = parseHttpLogger("")
	}

	// log file
	if v := firstNonEmpty(cliValue(cli, "log-file"), os.Getenv("GOFS_LOG_FILE")); v != "" {
		args.logFile = v
	}

	// compress
	if v := firstNonEmpty(cliValue(cli, "compress"), os.Getenv("GOFS_COMPRESS")); v != "" {
		c, err := parseCompress(v)
		if err != nil {
			return nil, err
		}
		args.compress = c
	}

	// tls
	if v := firstNonEmpty(cliValue(cli, "tls-cert"), os.Getenv("GOFS_TLS_CERT")); v != "" {
		args.tlsCert = v
	}
	if v := firstNonEmpty(cliValue(cli, "tls-key"), os.Getenv("GOFS_TLS_KEY")); v != "" {
		args.tlsKey = v
	}
	if (args.tlsCert == "") != (args.tlsKey == "") {
		if args.tlsCert == "" {
			return nil, fmt.Errorf("no tls-cert set")
		}
		return nil, fmt.Errorf("no tls-key set")
	}

	return &parseResult{args: args}, nil
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
	return splitRules([]string{v})
}

func sanitizePath(p string) (string, error) {
	if !fileExists(p) && !dirExists(p) {
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
	if !fileExists(filepath.Join(canon, "index.html")) {
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
			out = append(out, BindAddr{addr: a, isIP: true})
		} else {
			// unix socket path
			out = append(out, BindAddr{addr: a, isIP: false})
		}
	}
	return out, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
