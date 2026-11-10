package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfg, []byte(`
serve-path: .
bind: 127.0.0.1
port: 3000
allow-upload: true
allow-delete: true
hidden:
  - tmp
  - '*.log'
auth:
  - admin:admin@/:rw
  - '@/'
compress: high
`), 0o644)

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
