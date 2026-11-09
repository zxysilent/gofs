package main

import (
	"reflect"
	"testing"
)

func TestSha512Crypt(t *testing.T) {
	// Known vector from the dufs README (password "123456").
	stored := "$6$tWMB51u6Kb2ui3wd$5gVHP92V9kZcMwQeKTjyTRgySsYJu471Jb1I6iHQ8iZ6s07GgCIO69KcPBRuwPE5tDq05xMAzye0NxVKuJdYs/"
	if !verifySha512Crypt("123456", stored) {
		t.Fatalf("sha512 crypt verification failed for %q", "123456")
	}
	if verifySha512Crypt("wrong", stored) {
		t.Fatalf("sha512 crypt verification should fail for wrong password")
	}
	// A rounds-specified vector.
	roundsStored := "$6$rounds=10000$saltstringsaltst$OW1/O6BYHV6BcXZu8QVeXbDWra3Oeqh0sbHbbMCVNSnCM/UrjmM0Dp8vOuZeHBy/YTBmSK6H9qs/y3RnOaw5v."
	if !verifySha512Crypt("Hello world!", roundsStored) {
		t.Fatalf("sha512 crypt rounds verification failed")
	}
}

func TestNonceRoundTrip(t *testing.T) {
	nonce := createNonce()
	if len(nonce) != 34 {
		t.Fatalf("nonce length = %d, want 34", len(nonce))
	}
	valid, err := validateNonce(nonce)
	if err != nil || !valid {
		t.Fatalf("fresh nonce should validate: valid=%v err=%v", valid, err)
	}
	if _, err := validateNonce("deadbeefdeadbeefdeadbeefdeadbeef"); err == nil {
		t.Fatalf("invalid nonce should not validate")
	}
}

func TestDigestAuth(t *testing.T) {
	// Manually construct a digest Authorization header and verify checkAuth.
	// HA1 = md5("admin:DUFS:123"), nonce valid, qop=auth.
	nonce := createNonce()
	uri := "/secret.txt"
	ha1 := md5Hex("admin:DUFS:123")
	ha2 := md5Hex("GET:" + uri)
	cnonce := "0a4f113b"
	nc := "00000001"
	response := md5Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":auth:" + ha2)
	header := `Digest username="admin", realm="DUFS", nonce="` + nonce + `", uri="` + uri + `", qop=auth, nc=` + nc + `, cnonce="` + cnonce + `", response="` + response + `"`
	if !checkAuth(header, "GET", "admin", "123") {
		t.Fatalf("digest auth should succeed")
	}
	if checkAuth(header, "GET", "admin", "wrong") {
		t.Fatalf("digest auth should fail for wrong password")
	}
}

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

func TestSplitAccountPaths(t *testing.T) {
	cases := []struct {
		in      string
		account string
		paths   string
		ok      bool
	}{
		{"user:pass@/:rw", "user:pass", "/:rw", true},
		{"user:pass@@/:rw", "user:pass@", "/:rw", true},
		{"user:pass@1@/:rw", "user:pass@1", "/:rw", true},
		{"nope", "", "", false},
	}
	for _, c := range cases {
		a, p, ok := splitAccountPaths(c.in)
		if a != c.account || p != c.paths || ok != c.ok {
			t.Errorf("splitAccountPaths(%q) = (%q, %q, %v), want (%q, %q, %v)", c.in, a, p, ok, c.account, c.paths, c.ok)
		}
	}
}

func TestSplitRules(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"user1:pass1@/:rw|user2:pass2@/:rw"}, []string{"user1:pass1@/:rw", "user2:pass2@/:rw"}},
		{[]string{"user1:pa|ss1@/:rw|user2:pa|ss2@/:rw"}, []string{"user1:pa|ss1@/:rw", "user2:pa|ss2@/:rw"}},
		{[]string{"user1:pa|ss1@/:rw|@/"}, []string{"user1:pa|ss1@/:rw", "@" + "/"}},
	}
	for _, c := range cases {
		if got := splitRules(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitRules(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAccessPaths(t *testing.T) {
	paths := newAccessPaths(permIndexOnly)
	paths.add("/dir1", permReadWrite)
	paths.add("/dir2/dir21", permReadWrite)
	paths.add("/dir2/dir21/dir211", permReadOnly)
	paths.add("/dir2/dir22", permReadOnly)
	paths.add("/dir2/dir22/dir221", permReadWrite)
	paths.add("/dir2/dir23/dir231", permReadWrite)

	got := paths.entryPaths("/tmp")
	want := []string{"/tmp/dir1", "/tmp/dir2/dir21", "/tmp/dir2/dir22", "/tmp/dir2/dir23/dir231"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entryPaths = %v, want %v", got, want)
	}

	if v, ok := paths.find("dir1/file"); !ok || !reflect.DeepEqual(v, newAccessPaths(permReadWrite)) {
		t.Errorf("find(dir1/file) = %v, %v", v, ok)
	}
	if v, ok := paths.find("dir2/dir23/file"); ok || v != nil {
		t.Errorf("find(dir2/dir23/file) should be none, got %v, %v", v, ok)
	}
	if v, ok := paths.find("dir2/dir23//dir231/file"); !ok || !reflect.DeepEqual(v, newAccessPaths(permReadWrite)) {
		t.Errorf("find(dir2/dir23//dir231/file) = %v, %v", v, ok)
	}
}
