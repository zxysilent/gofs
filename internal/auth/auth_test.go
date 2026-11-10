package auth

import (
	"reflect"
	"testing"
)

func TestSha512Crypt(t *testing.T) {
	// Known sha512-crypt vector (password "123456").
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
	nonce := CreateNonce()
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
	// HA1 = md5("admin:GOFS:123"), nonce valid, qop=auth.
	nonce := CreateNonce()
	uri := "/secret.txt"
	ha1 := md5Hex("admin:" + realm + ":123")
	ha2 := md5Hex("GET:" + uri)
	cnonce := "0a4f113b"
	nc := "00000001"
	response := md5Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":auth:" + ha2)
	header := `Digest username="admin", realm="` + realm + `", nonce="` + nonce + `", uri="` + uri + `", qop=auth, nc=` + nc + `, cnonce="` + cnonce + `", response="` + response + `"`
	if !checkAuth(header, "GET", "admin", "123") {
		t.Fatalf("digest auth should succeed")
	}
	if checkAuth(header, "GET", "admin", "wrong") {
		t.Fatalf("digest auth should fail for wrong password")
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
		if got := SplitRules(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitRules(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAccessPaths(t *testing.T) {
	paths := NewAccessPaths(PermIndexOnly)
	paths.add("/dir1", PermReadWrite)
	paths.add("/dir2/dir21", PermReadWrite)
	paths.add("/dir2/dir21/dir211", PermReadOnly)
	paths.add("/dir2/dir22", PermReadOnly)
	paths.add("/dir2/dir22/dir221", PermReadWrite)
	paths.add("/dir2/dir23/dir231", PermReadWrite)

	got := paths.EntryPaths("/tmp")
	want := []string{"/tmp/dir1", "/tmp/dir2/dir21", "/tmp/dir2/dir22", "/tmp/dir2/dir23/dir231"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entryPaths = %v, want %v", got, want)
	}

	if v, ok := paths.find("dir1/file"); !ok || !reflect.DeepEqual(v, NewAccessPaths(PermReadWrite)) {
		t.Errorf("find(dir1/file) = %v, %v", v, ok)
	}
	if v, ok := paths.find("dir2/dir23/file"); ok || v != nil {
		t.Errorf("find(dir2/dir23/file) should be none, got %v, %v", v, ok)
	}
	if v, ok := paths.find("dir2/dir23//dir231/file"); !ok || !reflect.DeepEqual(v, NewAccessPaths(PermReadWrite)) {
		t.Errorf("find(dir2/dir23//dir231/file) = %v, %v", v, ok)
	}
}
