package config

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns the text.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(out)
}

// 帮助里的选项表是从 FlagSet 生成的，所以每个注册的选项都必须出现。
func TestPrintHelpListsEveryFlag(t *testing.T) {
	out := captureStdout(t, PrintHelp)

	f := &flags{}
	fs := newFlagSet(f)
	fs.VisitAll(func(fl *pflag.Flag) {
		if !strings.Contains(out, "--"+fl.Name) {
			t.Errorf("help is missing --%s", fl.Name)
		}
		if fl.Shorthand != "" && !strings.Contains(out, "-"+fl.Shorthand+",") {
			t.Errorf("help is missing the -%s shorthand of --%s", fl.Shorthand, fl.Name)
		}
	})
}

// 值占位符与布尔写法：包装 pflag.Value 后容易丢掉类型列或多出 "(default [])"。
func TestPrintHelpFormatting(t *testing.T) {
	out := captureStdout(t, PrintHelp)

	for _, want := range []string{
		"-c, --config <file>",
		"-b, --bind <addrs>",
		"-p, --port <port>",
		"-a, --auth <rules>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help should contain %q", want)
		}
	}
	// 空列表的默认值不该被打印成 "(default [])"。
	if strings.Contains(out, "(default [])") {
		t.Error("help should not show the empty slice default")
	}
	if !strings.Contains(out, "[serve-path]") {
		t.Error("help should document the positional argument")
	}
}

// 补全脚本的选项列表同样来自 FlagSet，短选项也要覆盖到。
func TestPrintCompletionsBash(t *testing.T) {
	out := captureStdout(t, func() { PrintCompletions("bash") })

	if !strings.Contains(out, "complete -F _gofs gofs") {
		t.Fatalf("not a bash completion script: %s", out)
	}
	// 取出 opts="..." 里的候选词，按空格切分后逐个比对。
	var words map[string]bool
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "local opts=\"") {
			continue
		}
		raw := strings.TrimSuffix(strings.TrimPrefix(line, "local opts=\""), "\"")
		words = map[string]bool{}
		for _, w := range strings.Fields(raw) {
			words[w] = true
		}
	}
	if words == nil {
		t.Fatalf("no opts list in the completion script: %s", out)
	}

	f := &flags{}
	fs := newFlagSet(f)
	fs.VisitAll(func(fl *pflag.Flag) {
		if !words["--"+fl.Name] {
			t.Errorf("completion is missing --%s", fl.Name)
		}
		if fl.Shorthand != "" && !words["-"+fl.Shorthand] {
			t.Errorf("completion is missing the -%s shorthand", fl.Shorthand)
		}
	})
}

func TestPrintCompletionsUnsupportedShell(t *testing.T) {
	out := captureStdout(t, func() { PrintCompletions("zsh") })
	if !strings.Contains(out, "not implemented") {
		t.Errorf("unexpected output for zsh: %s", out)
	}
}
