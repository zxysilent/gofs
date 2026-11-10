package auth

import (
	"crypto/ed25519"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// nonceStarHash is the md5 state prefix used to build digest-auth nonces
// (derived from a random seed plus the process id).
var nonceStarHash = computeNonceStar()

func computeNonceStar() []byte {
	seed := make([]byte, 16)
	if _, err := rand.Read(seed); err != nil {
		now := time.Now().UnixNano()
		for i := range seed {
			seed[i] = byte(now >> (8 * i))
		}
	}
	h := md5.New()
	h.Write(seed)
	h.Write([]byte(strconv.Itoa(os.Getpid())))
	return h.Sum(nil)
}

const (
	realm             = "GOFS"
	digestAuthTimeout = 60 * 60 * 24 * 7 // 7 days
	tokenExpiration   = 1000 * 60 * 60 * 24 * 3
	tokenSigLen       = ed25519.SignatureSize
	tokenExpLen       = 8
)

// AccessPerm models the three permission levels.
type AccessPerm int

const (
	PermIndexOnly AccessPerm = iota
	PermReadOnly
	PermReadWrite
)

func (p AccessPerm) IndexOnly() bool { return p == PermIndexOnly }
func (p AccessPerm) ReadWrite() bool { return p == PermReadWrite }

// read reports whether the permission allows fetching file contents
// (both read-only and read-write do; index-only does not).
func (p AccessPerm) Read() bool { return p != PermIndexOnly }

// maxPerm returns the higher of the two permissions.
func maxPerm(a, b AccessPerm) AccessPerm {
	if b > a {
		return b
	}
	return a
}

// AccessPaths is a trie of path -> permission entries.
type AccessPaths struct {
	Perm     AccessPerm
	children map[string]*AccessPaths
}

func NewAccessPaths(perm AccessPerm) *AccessPaths {
	return &AccessPaths{Perm: perm, children: map[string]*AccessPaths{}}
}

func (ap *AccessPaths) setPerm(perm AccessPerm) {
	if !perm.IndexOnly() {
		ap.Perm = perm
	}
}

// merge parses a comma-separated list of "/path[:ro|rw]" entries.
func (ap *AccessPaths) merge(paths string) bool {
	trimmed := strings.Trim(paths, ",")
	if trimmed == "" {
		return true
	}
	for _, item := range strings.Split(trimmed, ",") {
		var p string
		var perm AccessPerm
		if idx := strings.IndexByte(item, ':'); idx >= 0 {
			p = item[:idx]
			switch item[idx+1:] {
			case "ro":
				perm = PermReadOnly
			case "rw":
				perm = PermReadWrite
			default:
				return false
			}
		} else {
			p = item
			perm = PermReadOnly
		}
		ap.add(p, perm)
	}
	return true
}

func (ap *AccessPaths) add(p string, perm AccessPerm) {
	p = strings.Trim(p, "/")
	if p == "" {
		ap.setPerm(perm)
		return
	}
	ap.addImpl(strings.Split(p, "/"), perm)
}

func (ap *AccessPaths) addImpl(parts []string, perm AccessPerm) {
	if len(parts) == 0 {
		ap.Perm = perm
		return
	}
	child := ap.children[parts[0]]
	if child == nil {
		child = NewAccessPaths(PermIndexOnly)
		ap.children[parts[0]] = child
	}
	child.addImpl(parts[1:], perm)
}

// guard returns the access paths guarding path for method, or nil.
func (ap *AccessPaths) Guard(path, method string) *AccessPaths {
	target, ok := ap.find(path)
	if !ok {
		return nil
	}
	if !isReadonlyMethod(method) && !target.Perm.ReadWrite() {
		return nil
	}
	return target
}

func (ap *AccessPaths) find(p string) (*AccessPaths, bool) {
	trimmed := strings.Trim(p, "/")
	var parts []string
	if trimmed != "" {
		for _, seg := range strings.Split(trimmed, "/") {
			if seg != "" {
				parts = append(parts, seg)
			}
		}
	}
	return ap.findImpl(parts, ap.Perm)
}

func (ap *AccessPaths) findImpl(parts []string, perm AccessPerm) (*AccessPaths, bool) {
	if !ap.Perm.IndexOnly() {
		perm = ap.Perm
	}
	if len(parts) == 0 {
		if perm.IndexOnly() {
			return ap.clone(), true
		}
		return NewAccessPaths(perm), true
	}
	child, ok := ap.children[parts[0]]
	if !ok {
		if perm.IndexOnly() {
			return nil, false
		}
		return NewAccessPaths(perm), true
	}
	return child.findImpl(parts[1:], perm)
}

func (ap *AccessPaths) clone() *AccessPaths {
	c := NewAccessPaths(ap.Perm)
	for k, v := range ap.children {
		c.children[k] = v.clone()
	}
	return c
}

func (ap *AccessPaths) ChildNames() []string {
	names := make([]string, 0, len(ap.children))
	for k := range ap.children {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

// entryPaths returns the concrete paths that are directly accessible (where the
// permission is not index-only).
func (ap *AccessPaths) EntryPaths(base string) []string {
	if !ap.Perm.IndexOnly() {
		return []string{base}
	}
	var out []string
	ap.entryPathsImpl(&out, base)
	return out
}

func (ap *AccessPaths) entryPathsImpl(out *[]string, base string) {
	for _, name := range ap.ChildNames() {
		child := ap.children[name]
		b := base + "/" + name
		if child.Perm.IndexOnly() {
			child.entryPathsImpl(out, b)
		} else {
			*out = append(*out, b)
		}
	}
}

// absorbAnon merges an anonymous AccessPaths into a user's, "higher perm wins".
func (ap *AccessPaths) absorbAnon(anon, origUser *AccessPaths, userInherited, anonInherited AccessPerm) {
	anonEff := anon.Perm
	if anonEff.IndexOnly() {
		anonEff = anonInherited
	}
	origUserEff := origUser.Perm
	if origUserEff.IndexOnly() {
		origUserEff = userInherited
	}
	combined := maxPerm(anonEff, origUserEff)
	if !combined.IndexOnly() && combined > ap.Perm {
		ap.Perm = combined
	}
	defaultAP := NewAccessPaths(PermIndexOnly)
	for name, anonChild := range anon.children {
		origUserChild := origUser.children[name]
		if origUserChild == nil {
			origUserChild = defaultAP
		}
		userChild := ap.children[name]
		if userChild == nil {
			userChild = NewAccessPaths(PermIndexOnly)
			ap.children[name] = userChild
		}
		userChild.absorbAnon(anonChild, origUserChild, origUserEff, anonEff)
	}
}

type userEntry struct {
	pass  string
	paths AccessPaths
}

// AccessControl holds the account based access rules.
type AccessControl struct {
	empty             bool
	UseHashedPassword bool
	users             map[string]userEntry
	anonymous         *AccessPaths
}

func NewAccessControl(rawRules []string) (AccessControl, error) {
	ac := AccessControl{
		empty:     true,
		anonymous: NewAccessPaths(PermReadWrite),
		users:     map[string]userEntry{},
	}
	if len(rawRules) == 0 {
		return ac, nil
	}

	var annoyPaths *string
	var accountPaths [][3]string // user, pass, paths

	for _, rule := range SplitRules(rawRules) {
		account, paths, ok := splitAccountPaths(rule)
		if !ok {
			return ac, fmt.Errorf("invalid auth `%s`", rule)
		}
		if account == "" {
			if annoyPaths != nil {
				return ac, errors.New("invalid auth, no duplicate anonymous rules")
			}
			annoyPaths = &paths
		} else {
			user, pass, ok := strings.Cut(account, ":")
			if !ok || user == "" || pass == "" {
				return ac, fmt.Errorf("invalid auth `%s`", rule)
			}
			accountPaths = append(accountPaths, [3]string{user, pass, paths})
		}
	}

	var anonymous *AccessPaths
	if annoyPaths != nil {
		ap := NewAccessPaths(PermIndexOnly)
		if !ap.merge(*annoyPaths) {
			return ac, fmt.Errorf("invalid auth value `@%s`", *annoyPaths)
		}
		anonymous = ap
	}

	UseHashedPassword := false
	for _, t := range accountPaths {
		user, pass, paths := t[0], t[1], t[2]
		ap := NewAccessPaths(PermIndexOnly)
		if !ap.merge(paths) {
			return ac, fmt.Errorf("invalid auth value `%s:%s@%s`", user, pass, paths)
		}
		if anonymous != nil {
			orig := ap.clone()
			ap.absorbAnon(anonymous, orig, PermIndexOnly, PermIndexOnly)
		}
		if strings.HasPrefix(pass, "$6$") {
			UseHashedPassword = true
		}
		ac.users[user] = userEntry{pass: pass, paths: *ap}
	}

	ac.empty = false
	ac.UseHashedPassword = UseHashedPassword
	ac.anonymous = anonymous
	return ac, nil
}

func (ac *AccessControl) HasUsers() bool { return len(ac.users) > 0 }

// guard authenticates a request and returns (user, accessPaths). A nil
// accessPaths means "no access": empty user => reject(401), non-empty => forbid.
func (ac *AccessControl) Guard(path, method, authHeader, token string, guardOptions bool) (string, *AccessPaths) {
	if ac.empty {
		return "", NewAccessPaths(PermReadWrite)
	}

	if method == "GET" && token != "" {
		if user, ap, err := ac.VerifyToken(token, path); err == nil {
			return user, ap.Guard(path, method)
		}
	}

	if authHeader != "" {
		if user := getAuthUser(authHeader); user != "" {
			if entry, ok := ac.users[user]; ok {
				if method == "OPTIONS" {
					return user, NewAccessPaths(PermReadOnly)
				}
				if checkAuth(authHeader, method, user, entry.pass) {
					return user, entry.paths.Guard(path, method)
				}
			}
		}
		return "", nil
	}

	if !guardOptions && method == "OPTIONS" {
		return "", NewAccessPaths(PermReadOnly)
	}

	if ac.anonymous != nil {
		return "", ac.anonymous.Guard(path, method)
	}
	return "", nil
}

func (ac *AccessControl) GenerateToken(path, user string) (string, error) {
	entry, ok := ac.users[user]
	if !ok {
		return "", fmt.Errorf("not found user '%s'", user)
	}
	exp := uint64(time.Now().UnixMilli()) + tokenExpiration
	message := fmt.Sprintf("%s:%d", path, exp)
	seed := sha256.Sum256([]byte(user + ":" + entry.pass))
	key := ed25519.NewKeyFromSeed(seed[:])
	sig := ed25519.Sign(key, []byte(message))
	raw := make([]byte, 0, tokenSigLen+tokenExpLen+len(user))
	raw = append(raw, sig...)
	expB := make([]byte, tokenExpLen)
	binary.BigEndian.PutUint64(expB, exp)
	raw = append(raw, expB...)
	raw = append(raw, user...)
	return hex.EncodeToString(raw), nil
}

func (ac *AccessControl) VerifyToken(token, path string) (string, *AccessPaths, error) {
	raw, err := hex.DecodeString(token)
	if err != nil {
		return "", nil, err
	}
	if len(raw) < tokenSigLen+tokenExpLen {
		return "", nil, errors.New("invalid token")
	}
	sig := raw[:tokenSigLen]
	exp := binary.BigEndian.Uint64(raw[tokenSigLen : tokenSigLen+tokenExpLen])
	user := string(raw[tokenSigLen+tokenExpLen:])
	if uint64(time.Now().UnixMilli()) > exp {
		return "", nil, errors.New("token expired")
	}
	entry, ok := ac.users[user]
	if !ok {
		return "", nil, fmt.Errorf("not found user '%s'", user)
	}
	message := fmt.Sprintf("%s:%d", path, exp)
	seed := sha256.Sum256([]byte(user + ":" + entry.pass))
	pub := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	if !ed25519.Verify(pub, []byte(message), sig) {
		return "", nil, errors.New("invalid signature")
	}
	return user, &entry.paths, nil
}

func isReadonlyMethod(method string) bool {
	switch method {
	case "GET", "OPTIONS", "HEAD", "PROPFIND", "CHECKAUTH", "LOGOUT":
		return true
	}
	return false
}

// getAuthUser extracts the username from a Basic or Digest Authorization header.
func getAuthUser(authorization string) string {
	if v, ok := strings.CutPrefix(authorization, "Basic "); ok {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return ""
		}
		if user, _, ok := strings.Cut(string(decoded), ":"); ok {
			return user
		}
		return ""
	}
	if v, ok := strings.CutPrefix(authorization, "Digest "); ok {
		m, ok := toHeaderMap(v)
		if !ok {
			return ""
		}
		return m["username"]
	}
	return ""
}

// checkAuth validates a Basic or Digest authorization header.
func checkAuth(authorization, method, authUser, authPass string) bool {
	if v, ok := strings.CutPrefix(authorization, "Basic "); ok {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return false
		}
		user, pass, ok := strings.Cut(string(decoded), ":")
		if !ok || user != authUser {
			return false
		}
		if strings.HasPrefix(authPass, "$6$") {
			return verifySha512Crypt(pass, authPass)
		}
		return constantTimeEq(pass, authPass)
	}
	if v, ok := strings.CutPrefix(authorization, "Digest "); ok {
		m, ok := toHeaderMap(v)
		if !ok {
			return false
		}
		username := m["username"]
		nonce := m["nonce"]
		userResponse := m["response"]
		if username == "" || nonce == "" || userResponse == "" {
			return false
		}
		if valid, _ := validateNonce(nonce); !valid {
			return false
		}
		if authUser != username {
			return false
		}
		// HA1 = MD5(user:realm:pass)
		ha1 := md5Hex(authUser + ":" + realm + ":" + authPass)
		// HA2 = MD5(method:uri)
		ha2 := md5Hex(method + ":" + m["uri"])
		var correct string
		if qop := m["qop"]; qop == "auth" || qop == "auth-int" {
			correct = md5Hex(ha1 + ":" + nonce + ":" + m["nc"] + ":" + m["cnonce"] + ":" + qop + ":" + ha2)
		} else {
			correct = md5Hex(ha1 + ":" + nonce + ":" + ha2)
		}
		return constantTimeEq(correct, userResponse)
	}
	return false
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func constantTimeEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// toHeaderMap parses a "key=value, key=\"value\"" digest header.
func toHeaderMap(header string) (map[string]string, bool) {
	m := map[string]string{}
	for _, part := range splitDigest(header) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq <= 0 {
			return nil, false
		}
		key := strings.TrimSpace(part[:eq])
		val := strings.Trim(part[eq+1:], `"`)
		m[key] = val
	}
	return m, true
}

func splitDigest(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
			cur.WriteByte(c)
		case c == ',' && !inQuote:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// CreateNonce builds a digest auth nonce.
func CreateNonce() string {
	secs := uint32(time.Now().Unix())
	return fmt.Sprintf("%08x%x", secs, nonceSuffix(secs))[:34]
}

// nonceSuffix returns the 32 hex-char md5 hash for a nonce's seconds value.
func nonceSuffix(secs uint32) []byte {
	h := md5.New()
	h.Write(nonceStarHash)
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, secs)
	h.Write(b)
	return h.Sum(nil)
}

// validateNonce reports whether a nonce is still valid.
func validateNonce(nonce string) (bool, error) {
	if len(nonce) != 34 {
		return false, errors.New("invalid nonce")
	}
	secs, err := strconv.ParseUint(nonce[:8], 16, 32)
	if err != nil {
		return false, errors.New("invalid nonce")
	}
	secsU32 := uint32(secs)
	now := uint32(time.Now().Unix())
	if now < secsU32 {
		return false, errors.New("invalid nonce")
	}
	sum := hex.EncodeToString(nonceSuffix(secsU32))
	if sum[:26] != nonce[8:34] {
		return false, errors.New("invalid nonce")
	}
	return now-secsU32 < digestAuthTimeout, nil
}

// splitAccountPaths splits "account@/paths" on the first "@/".
func splitAccountPaths(s string) (account, paths string, ok bool) {
	idx := strings.Index(s, "@/")
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}

// splitRules splits "|"-separated auth rules, keeping "|" that belongs to a
// password (i.e. before the "@/..." marker).
func SplitRules(rules []string) []string {
	var out []string
	for _, rule := range rules {
		parts := strings.Split(rule, "|")
		var rulesList []string
		concat := ""
		for i, part := range parts {
			if strings.Contains(part, "@/") {
				concat += part
				rulesList = append(rulesList, concat)
				concat = ""
				continue
			}
			concat += part
			if i < len(parts)-1 {
				concat += "|"
			}
		}
		if concat != "" {
			rulesList = append(rulesList, concat)
		}
		out = append(out, rulesList...)
	}
	return out
}

// ---- SHA-512 crypt (for hashed passwords) ----

const cryptB64 = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// verifySha512Crypt verifies password against a "$6$..." hash.
func verifySha512Crypt(password, stored string) bool {
	salt, rounds, expected, ok := parseCryptSHA512(stored)
	if !ok {
		return false
	}
	return constantTimeEq(cryptSHA512(password, salt, rounds), expected)
}

func parseCryptSHA512(stored string) (salt string, rounds int, hash string, ok bool) {
	if !strings.HasPrefix(stored, "$6$") {
		return "", 0, "", false
	}
	rest := stored[3:]
	rounds = 5000
	if strings.HasPrefix(rest, "rounds=") {
		idx := strings.IndexByte(rest, '$')
		if idx < 0 {
			return "", 0, "", false
		}
		r, err := strconv.Atoi(rest[len("rounds="):idx])
		if err != nil {
			return "", 0, "", false
		}
		rounds = r
		rest = rest[idx+1:]
	}
	idx := strings.LastIndexByte(rest, '$')
	if idx < 0 {
		return "", 0, "", false
	}
	salt = rest[:idx]
	hash = rest[idx+1:]
	if len(salt) > 16 || hash == "" {
		return "", 0, "", false
	}
	return salt, rounds, hash, true
}

// cryptSHA512 implements Ulrich Drepper's SHA-512 crypt hashing and returns the
// 86 character encoded hash.
func cryptSHA512(password, salt string, rounds int) string {
	key := []byte(password)
	saltB := []byte(salt)
	keyLen := len(key)
	saltLen := len(saltB)

	// 1. sumB = sha512(key + salt + key)
	h := sha512.New()
	h.Write(key)
	h.Write(saltB)
	h.Write(key)
	sumB := h.Sum(nil)

	// 2. sumA = sha512(key + salt + Repeat(sumB, keyLen)) with the P sequence.
	h = sha512.New()
	h.Write(key)
	h.Write(saltB)
	h.Write(repeatBytes(sumB, keyLen))
	for i := keyLen; i > 0; i >>= 1 {
		if i%2 == 0 {
			h.Write(key)
		} else {
			h.Write(sumB)
		}
	}
	sumA := h.Sum(nil)

	// 3. seqP = Repeat(sha512(key * keyLen), keyLen)
	h = sha512.New()
	for i := 0; i < keyLen; i++ {
		h.Write(key)
	}
	seqP := repeatBytes(h.Sum(nil), keyLen)

	// 4. seqS = Repeat(sha512(salt * (16 + sumA[0])), saltLen)
	h = sha512.New()
	for i := 0; i < 16+int(sumA[0]); i++ {
		h.Write(saltB)
	}
	seqS := repeatBytes(h.Sum(nil), saltLen)

	// 5. rounds loop.
	for i := 0; i < rounds; i++ {
		h = sha512.New()
		if i&1 != 0 {
			h.Write(seqP)
		} else {
			h.Write(sumA)
		}
		if i%3 != 0 {
			h.Write(seqS)
		}
		if i%7 != 0 {
			h.Write(seqP)
		}
		if i&1 != 0 {
			h.Write(sumA)
		} else {
			h.Write(seqP)
		}
		sumA = h.Sum(nil)
	}

	// 6. encode 64 bytes into 86 crypt-base64 characters.
	var src [64]byte
	for i, idx := range sha512Perm {
		src[i] = sumA[idx]
	}
	return string(base64_24bit(src[:]))
}

// sha512Perm is the byte permutation used when encoding the SHA-512 crypt
// digest (from the reference implementation).
var sha512Perm = [64]int{
	42, 21, 0,
	1, 43, 22,
	23, 2, 44,
	45, 24, 3,
	4, 46, 25,
	26, 5, 47,
	48, 27, 6,
	7, 49, 28,
	29, 8, 50,
	51, 30, 9,
	10, 52, 31,
	32, 11, 53,
	54, 33, 12,
	13, 55, 34,
	35, 14, 56,
	57, 36, 15,
	16, 58, 37,
	38, 17, 59,
	60, 39, 18,
	19, 61, 40,
	41, 20, 62,
	63,
}

// repeatBytes repeats input to fill a slice of the given length.
func repeatBytes(input []byte, length int) []byte {
	out := make([]byte, length)
	if len(input) == 0 {
		return out
	}
	for i := range out {
		out[i] = input[i%len(input)]
	}
	return out
}

// base64_24bit encodes bytes using the crypt base64 alphabet (no padding).
func base64_24bit(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dstlen := (len(src)*8 + 5) / 6
	dst := make([]byte, dstlen)
	di, si := 0, 0
	n := len(src) / 3 * 3
	for si < n {
		val := uint(src[si+2])<<16 | uint(src[si+1])<<8 | uint(src[si])
		dst[di] = cryptB64[val&0x3f]
		dst[di+1] = cryptB64[val>>6&0x3f]
		dst[di+2] = cryptB64[val>>12&0x3f]
		dst[di+3] = cryptB64[val>>18]
		di += 4
		si += 3
	}
	rem := len(src) - si
	if rem == 0 {
		return dst
	}
	val := uint(src[si])
	if rem == 2 {
		val |= uint(src[si+1]) << 8
	}
	dst[di] = cryptB64[val&0x3f]
	dst[di+1] = cryptB64[val>>6&0x3f]
	if rem == 2 {
		dst[di+2] = cryptB64[val>>12]
	}
	return dst
}
