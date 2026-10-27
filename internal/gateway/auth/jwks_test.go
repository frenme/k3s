package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func jwkFor(kid string, pub *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func signRS(t *testing.T, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims())
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

type idp struct {
	srv   *httptest.Server
	keys  atomic.Pointer[[]map[string]string]
	calls atomic.Int32
}

func newIDP(t *testing.T, initial ...map[string]string) *idp {
	t.Helper()
	p := &idp{}
	p.keys.Store(&initial)
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": *p.keys.Load()})
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func TestJWKSVerifiesAndFollowsKeyRotation(t *testing.T) {
	k1, _ := rsa.GenerateKey(rand.Reader, 2048)
	k2, _ := rsa.GenerateKey(rand.Reader, 2048)
	p := newIDP(t, jwkFor("key-1", &k1.PublicKey))

	v, err := NewVerifier(JWTConfig{JWKSURL: p.srv.URL, JWKSRefresh: time.Hour}, discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := v.jwks.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if sub, err := v.Subject(signRS(t, k1, "key-1")); err != nil || sub != "user-1" {
		t.Fatalf("key-1: %q %v", sub, err)
	}
	callsBefore := p.calls.Load()
	if _, err := v.Subject(signRS(t, k1, "key-1")); err != nil || p.calls.Load() != callsBefore {
		t.Fatalf("a known kid must not trigger a fetch (calls %d -> %d)", callsBefore, p.calls.Load())
	}

	keys := []map[string]string{jwkFor("key-1", &k1.PublicKey), jwkFor("key-2", &k2.PublicKey)}
	p.keys.Store(&keys)
	v.jwks.now = func() time.Time { return time.Now().Add(time.Minute) }
	if sub, err := v.Subject(signRS(t, k2, "key-2")); err != nil || sub != "user-1" {
		t.Fatalf("a rotated key must be picked up: %q %v", sub, err)
	}
}

func TestJWKSRejectsUnknownKeysWithoutHammeringTheIDP(t *testing.T) {
	k1, _ := rsa.GenerateKey(rand.Reader, 2048)
	attacker, _ := rsa.GenerateKey(rand.Reader, 2048)
	p := newIDP(t, jwkFor("key-1", &k1.PublicKey))
	v, _ := NewVerifier(JWTConfig{JWKSURL: p.srv.URL, JWKSRefresh: time.Hour}, discard())
	if err := v.jwks.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := p.calls.Load()
	for range 20 {
		if _, err := v.Subject(signRS(t, attacker, "forged")); err == nil {
			t.Fatal("a token with an unknown kid was accepted")
		}
	}
	if got := p.calls.Load() - before; got != 0 {
		t.Fatalf("unknown kids caused %d fetches within the refresh gap", got)
	}
	if _, err := v.Subject(signRS(t, attacker, "key-1")); err == nil {
		t.Fatal("a token signed with another key but a known kid was accepted")
	}
}

func TestParseJWKSRejectsBrokenSets(t *testing.T) {
	for name, body := range map[string]string{
		"not json":  "nope",
		"empty":     `{"keys":[]}`,
		"bad rsa":   `{"keys":[{"kty":"RSA","kid":"a","n":"!!","e":"AQAB"}]}`,
		"unknown":   `{"keys":[{"kty":"oct","kid":"a"}]}`,
		"off curve": `{"keys":[{"kty":"EC","kid":"a","crv":"P-256","x":"AQ","y":"AQ"}]}`,
	} {
		if _, err := parseJWKS([]byte(body)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
