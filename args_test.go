package main

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
	if args.port != 3000 {
		t.Errorf("port = %d, want 3000", args.port)
	}
	if !args.allowUpload || !args.allowDelete {
		t.Errorf("allow flags not set: %+v", args)
	}
	if len(args.hidden) != 2 || args.hidden[0] != "tmp" || args.hidden[1] != "*.log" {
		t.Errorf("hidden = %v", args.hidden)
	}
	if !args.auth.hasUsers() {
		t.Errorf("auth should have users")
	}
	if args.compress != compressHigh {
		t.Errorf("compress = %v, want high", args.compress)
	}
	if len(args.addrs) != 1 || args.addrs[0].addr != "127.0.0.1" {
		t.Errorf("addrs = %+v", args.addrs)
	}
}

func TestParseArgsCLI(t *testing.T) {
	dir := t.TempDir()
	res, err := parseArgs([]string{"-A", "-p", "9999", "--hidden", "tmp,*.log", dir})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	args := res.args
	if args.port != 9999 {
		t.Errorf("port = %d, want 9999", args.port)
	}
	if !args.allowAll || !args.allowUpload || !args.allowDelete {
		t.Errorf("allow-all not propagated: %+v", args)
	}
	if len(args.hidden) != 2 {
		t.Errorf("hidden = %v", args.hidden)
	}
	canon, err := filepath.EvalSymlinks(dir)
	if err == nil && args.servePath != canon {
		t.Errorf("servePath = %q, want %q", args.servePath, canon)
	}
}

func TestParseArgsAuthRule(t *testing.T) {
	dir := t.TempDir()
	res, err := parseArgs([]string{"-a", "admin:123@/:rw", dir})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !res.args.auth.hasUsers() {
		t.Errorf("auth should have users")
	}
}
