package server

import (
	"reflect"
	"testing"
)

// 这两个测试覆盖的是 server/utils.go 里的 HTTP 辅助函数，之前误放在 auth 包里。

func TestParseRange(t *testing.T) {
	cases := []struct {
		in   string
		size uint64
		want [][2]uint64
	}{
		{"bytes=0-499", 500, [][2]uint64{{0, 499}}},
		{"bytes=0-", 500, [][2]uint64{{0, 499}}},
		{"bytes=299-", 500, [][2]uint64{{299, 499}}},
		{"bytes=-500", 500, [][2]uint64{{0, 499}}},
		{"bytes=-300", 500, [][2]uint64{{200, 499}}},
		{"bytes=0-199, 100-399, 400-, -200", 500, [][2]uint64{{0, 199}, {100, 399}, {400, 499}, {300, 499}}},
		{"bytes=500-", 500, nil},
		{"bytes=-501", 500, nil},
		{"bytes=0-500", 500, nil},
		{"bytes=0-199,", 500, nil},
		{"bytes=0-199, 500-", 500, nil},
	}
	for _, c := range cases {
		got := parseRange(c.in, c.size)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseRange(%q, %d) = %v, want %v", c.in, c.size, got, c.want)
		}
	}
}

func TestGlob(t *testing.T) {
	trues := [][2]string{
		{"", ""}, {".*", ".git"}, {"abc", "abc"}, {"a*c", "abc"},
		{"a?c", "abc"}, {"a*c", "abbc"}, {"*c", "abc"}, {"a*", "abc"},
		{"?c", "bc"}, {"a?", "ab"}, {"*.abc-cba", "xyz.abc-cba"},
		{"*.abc-cba", "123.xyz.abc-cba"}, {"*.log", ".log"}, {"*.log", "a.log"},
	}
	for _, c := range trues {
		if !glob(c[0], c[1]) {
			t.Errorf("glob(%q, %q) = false, want true", c[0], c[1])
		}
	}
	falses := [][2]string{
		{"abc", "adc"}, {"abc", "abcd"}, {"a?c", "abbc"}, {"*.log", "log"},
	}
	for _, c := range falses {
		if glob(c[0], c[1]) {
			t.Errorf("glob(%q, %q) = true, want false", c[0], c[1])
		}
	}
}
