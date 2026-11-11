package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadArgs 走与 ParseArgs 相同的两步：先填 options，再 resolve。测试只关心
// 结果，所以直接复用这条路径而不是复制一份合并逻辑。
func loadArgs(path string) (Args, error) {
	opts := defaultOptions()
	if err := loadConfig(path, &opts); err != nil {
		return Args{}, err
	}
	return opts.resolve()
}

func writeConfig(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	cfg := writeConfig(t, "config.toml", `
serve-path = "."
bind = ["127.0.0.1"]
port = 3000
path-prefix = "files"
allow-upload = true
allow-delete = true
hidden = ["tmp", "*.log"]
auth = ["admin:admin@/:rw", "@/"]
compress = "high"
log-level = "warn"
`)

	args, err := loadArgs(cfg)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if args.Port != 3000 {
		t.Errorf("port = %d, want 3000", args.Port)
	}
	if !args.AllowUpload || !args.AllowDelete {
		t.Errorf("allow flags not set: %+v", args)
	}
	if len(args.Hidden) != 2 || args.Hidden[0] != "tmp" || args.Hidden[1] != "*.log" {
		t.Errorf("hidden = %v", args.Hidden)
	}
	if !args.Auth.HasUsers() {
		t.Errorf("auth should have users")
	}
	if args.Compress != CompressHigh {
		t.Errorf("compress = %v, want high", args.Compress)
	}
	if len(args.Addrs) != 1 || args.Addrs[0].Addr != "127.0.0.1" {
		t.Errorf("addrs = %+v", args.Addrs)
	}
	if args.PathPrefix != "files" {
		t.Errorf("path-prefix = %q", args.PathPrefix)
	}
	if args.LogLevel != "warn" {
		t.Errorf("log-level = %q", args.LogLevel)
	}
}

// 缺省的键保持 DefaultArgs 的值，不会被零值覆盖。
func TestLoadConfigKeepsDefaults(t *testing.T) {
	cfg := writeConfig(t, "config.toml", `allow-search = true`)

	args, err := loadArgs(cfg)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if args.Port != DefaultArgs().Port {
		t.Errorf("port = %d, want the default %d", args.Port, DefaultArgs().Port)
	}
	if args.Compress != CompressLow {
		t.Errorf("compress = %v, want the default low", args.Compress)
	}
	if !args.AllowSearch || args.AllowUpload {
		t.Errorf("flags = %+v", args)
	}
}

// 拼错的键必须报错，否则配置看着生效其实没有。
func TestLoadConfigUnknownKey(t *testing.T) {
	cfg := writeConfig(t, "config.toml", `
port = 3000
allow-uplaod = true
`)

	_, err := loadArgs(cfg)
	if err == nil {
		t.Fatal("expected an error for an unknown key")
	}
	if !strings.Contains(err.Error(), "allow-uplaod") {
		t.Errorf("error should name the offending key: %v", err)
	}
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	cfg := writeConfig(t, "config.toml", "port = \n")

	if _, err := loadArgs(cfg); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestLoadConfigInvalidValues(t *testing.T) {
	cases := map[string]string{
		"port":     `port = 70000`,
		"compress": `compress = "extreme"`,
		"auth":     `auth = ["admin"]`,
	}
	for name, content := range cases {
		cfg := writeConfig(t, "config.toml", content)
		if _, err := loadArgs(cfg); err == nil {
			t.Errorf("%s: expected an error for %q", name, content)
		}
	}
}

func TestParseArgsCLI(t *testing.T) {
	dir := t.TempDir()
	res, err := ParseArgs([]string{"-A", "-p", "9999", "--hidden", "tmp,*.log", dir})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	args := res.Args
	if args.Port != 9999 {
		t.Errorf("port = %d, want 9999", args.Port)
	}
	if !args.AllowAll || !args.AllowUpload || !args.AllowDelete {
		t.Errorf("allow-all not propagated: %+v", args)
	}
	if len(args.Hidden) != 2 {
		t.Errorf("hidden = %v", args.Hidden)
	}
	canon, err := filepath.EvalSymlinks(dir)
	if err == nil && args.ServePath != canon {
		t.Errorf("servePath = %q, want %q", args.ServePath, canon)
	}
}

func TestParseArgsAuthRule(t *testing.T) {
	dir := t.TempDir()
	res, err := ParseArgs([]string{"-a", "admin:123@/:rw", dir})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if !res.Args.Auth.HasUsers() {
		t.Errorf("auth should have users")
	}
}

// 命令行参数优先于配置文件。
func TestParseArgsConfigOverriddenByCLI(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, "config.toml", `port = 3000`)

	res, err := ParseArgs([]string{"-c", cfg, "-p", "4000", dir})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if res.Args.Port != 4000 {
		t.Errorf("port = %d, want the CLI value 4000", res.Args.Port)
	}
}

// 各种 GNU/POSIX 写法都要能用 —— 这些正是换 pflag 的理由。
func TestParseArgsFlagForms(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name string
		argv []string
	}{
		{"separate value", []string{"-p", "9998"}},
		{"attached shorthand", []string{"-p9998"}},
		{"long with equals", []string{"--port=9998"}},
		{"long separate", []string{"--port", "9998"}},
	}
	for _, c := range cases {
		res, err := ParseArgs(append(c.argv, dir))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if res.Args.Port != 9998 {
			t.Errorf("%s: port = %d, want 9998", c.name, res.Args.Port)
		}
	}
}

// 位置参数可以出现在选项之间或之后。
func TestParseArgsInterspersedPositional(t *testing.T) {
	dir := t.TempDir()

	for _, argv := range [][]string{
		{dir, "-A"},
		{"-A", dir},
		{"--hidden", "tmp", dir, "-A"},
	} {
		res, err := ParseArgs(argv)
		if err != nil {
			t.Fatalf("ParseArgs(%v): %v", argv, err)
		}
		canon, err := filepath.EvalSymlinks(dir)
		if err == nil && res.Args.ServePath != canon {
			t.Errorf("argv %v: servePath = %q, want %q", argv, res.Args.ServePath, canon)
		}
		if !res.Args.AllowAll {
			t.Errorf("argv %v: -A not applied", argv)
		}
	}
}

// 短选项合并：-Ab 等价于 -A -b。
func TestParseArgsCombinedShorthand(t *testing.T) {
	dir := t.TempDir()
	res, err := ParseArgs([]string{"-Ab", "127.0.0.1", dir})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if len(res.Args.Addrs) != 1 || res.Args.Addrs[0].Addr != "127.0.0.1" {
		t.Errorf("addrs = %+v, want the single 127.0.0.1 bind", res.Args.Addrs)
	}
	if !res.Args.AllowAll {
		t.Errorf("-A in -Ab was not applied")
	}
}

// 重复的列表选项会累加，并且按逗号拆分。
func TestParseArgsRepeatedLists(t *testing.T) {
	dir := t.TempDir()
	res, err := ParseArgs([]string{
		"--hidden", "tmp,*.log", "--hidden", "*.lock",
		"-a", "admin:123@/:rw", "-a", "@/",
		"-b", "127.0.0.1,::1",
		dir,
	})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if len(res.Args.Hidden) != 3 {
		t.Errorf("hidden = %v, want 3 entries", res.Args.Hidden)
	}
	if len(res.Args.Addrs) != 2 {
		t.Errorf("addrs = %+v, want 2", res.Args.Addrs)
	}
	if !res.Args.Auth.HasUsers() {
		t.Errorf("auth should have users")
	}
}

// 布尔选项可以显式关闭（pflag 支持 --flag=false，手写解析器做不到）。
func TestParseArgsExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	res, err := ParseArgs([]string{"-A", "--allow-upload=false", dir})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	// -A 的连带效果晚于单选项，所以这里仍然是 true；关键是不报错。
	if !res.Args.AllowDelete {
		t.Errorf("-A should still enable delete")
	}
}

// 未知选项必须是错误。
func TestParseArgsUnknownFlag(t *testing.T) {
	dir := t.TempDir()
	_, err := ParseArgs([]string{"--nope", dir})
	if err == nil {
		t.Fatal("expected an error for an unknown flag")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should name the flag: %v", err)
	}
}

func TestParseArgsHelpVersion(t *testing.T) {
	if res, err := ParseArgs([]string{"-h"}); err != nil || !res.Help {
		t.Errorf("-h: res=%+v err=%v", res, err)
	}
	if res, err := ParseArgs([]string{"--help"}); err != nil || !res.Help {
		t.Errorf("--help: res=%+v err=%v", res, err)
	}
	if res, err := ParseArgs([]string{"-V"}); err != nil || !res.Version {
		t.Errorf("-V: res=%+v err=%v", res, err)
	}
	if res, err := ParseArgs([]string{"--version"}); err != nil || !res.Version {
		t.Errorf("--version: res=%+v err=%v", res, err)
	}
}

func TestParseArgsCompletions(t *testing.T) {
	res, err := ParseArgs([]string{"--completions", "bash"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if res.Completions != "bash" {
		t.Errorf("completions = %q", res.Completions)
	}
}

// -A 只覆盖写操作与查询能力，不能顺带打开符号链接沙箱。
func TestAllowAllDoesNotEnableSymlink(t *testing.T) {
	dir := t.TempDir()
	res, err := ParseArgs([]string{"-A", dir})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if !res.Args.AllowUpload || !res.Args.AllowDelete || !res.Args.AllowHash {
		t.Errorf("-A should enable write operations: %+v", res.Args)
	}
	if res.Args.AllowSymlink {
		t.Error("-A must not enable --allow-symlink")
	}

	// 显式给出时仍然生效。
	res, err = ParseArgs([]string{"-A", "--allow-symlink", dir})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if !res.Args.AllowSymlink {
		t.Error("--allow-symlink should still work explicitly")
	}
}
