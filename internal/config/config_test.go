package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	args, err := loadConfig(cfg)
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

// 列表既可以写成单个字符串，也可以写成数组。
func TestLoadConfigSingleStringList(t *testing.T) {
	cfg := writeConfig(t, "config.toml", `
bind = "127.0.0.1"
auth = "admin:123@/:rw"
`)

	args, err := loadConfig(cfg)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(args.Addrs) != 1 || args.Addrs[0].Addr != "127.0.0.1" {
		t.Errorf("addrs = %+v", args.Addrs)
	}
	if !args.Auth.HasUsers() {
		t.Errorf("auth should have users")
	}
}

// 缺省的键保持 DefaultArgs 的值，不会被零值覆盖。
func TestLoadConfigKeepsDefaults(t *testing.T) {
	cfg := writeConfig(t, "config.toml", `allow-search = true`)

	args, err := loadConfig(cfg)
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

	_, err := loadConfig(cfg)
	if err == nil {
		t.Fatal("expected an error for an unknown key")
	}
	if !strings.Contains(err.Error(), "allow-uplaod") {
		t.Errorf("error should name the offending key: %v", err)
	}
}

// 旧的 YAML 配置要给出明确提示，而不是一个 TOML 语法错误。
func TestLoadConfigRejectsYAML(t *testing.T) {
	cfg := writeConfig(t, "config.yaml", "port: 3000\n")

	_, err := loadConfig(cfg)
	if err == nil {
		t.Fatal("expected an error for a YAML config")
	}
	if !strings.Contains(err.Error(), "TOML") {
		t.Errorf("error should point at TOML: %v", err)
	}
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	cfg := writeConfig(t, "config.toml", "port = \n")

	if _, err := loadConfig(cfg); err == nil {
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
		if _, err := loadConfig(cfg); err == nil {
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
