package gateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "unit-test-secret"

func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func validClaims() jwt.RegisteredClaims {
	return jwt.RegisteredClaims{Subject: "user-1", Issuer: "idp", Audience: jwt.ClaimStrings{"api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}
}

func TestVerifierHMAC(t *testing.T) {
	v, err := NewVerifier(JWTConfig{Secret: testSecret, Issuer: "idp", Audience: "api"})
	if err != nil {
		t.Fatal(err)
	}
	expired := validClaims()
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	noExp := validClaims()
	noExp.ExpiresAt = nil
	noSub := validClaims()
	noSub.Subject = ""
	otherIssuer := validClaims()
	otherIssuer.Issuer = "evil"
	otherAudience := validClaims()
	otherAudience.Audience = jwt.ClaimStrings{"other"}

	if sub, err := v.Subject(sign(t, jwt.SigningMethodHS256, []byte(testSecret), validClaims())); err != nil || sub != "user-1" {
		t.Fatalf("valid token: %q %v", sub, err)
	}
	bad := map[string]string{
		"wrong secret":   sign(t, jwt.SigningMethodHS256, []byte("other"), validClaims()),
		"expired":        sign(t, jwt.SigningMethodHS256, []byte(testSecret), expired),
		"no expiry":      sign(t, jwt.SigningMethodHS256, []byte(testSecret), noExp),
		"no subject":     sign(t, jwt.SigningMethodHS256, []byte(testSecret), noSub),
		"wrong issuer":   sign(t, jwt.SigningMethodHS256, []byte(testSecret), otherIssuer),
		"wrong audience": sign(t, jwt.SigningMethodHS256, []byte(testSecret), otherAudience),
		"alg none":       sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims()),
		"garbage":        "not.a.jwt",
	}
	for name, token := range bad {
		if _, err := v.Subject(token); err == nil {
			t.Errorf("%s: token accepted", name)
		}
	}
}

func TestVerifierPublicKeyRejectsHMACConfusion(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	v, err := NewVerifier(JWTConfig{PublicKey: pemKey})
	if err != nil {
		t.Fatal(err)
	}
	if sub, err := v.Subject(sign(t, jwt.SigningMethodES256, priv, validClaims())); err != nil || sub != "user-1" {
		t.Fatalf("valid token: %q %v", sub, err)
	}
	if _, err := v.Subject(sign(t, jwt.SigningMethodHS256, der, validClaims())); err == nil {
		t.Fatal("HS256 token signed with the public key was accepted")
	}
}

func TestNewVerifierValidatesConfig(t *testing.T) {
	for name, cfg := range map[string]JWTConfig{
		"empty":    {},
		"both":     {Secret: "a", PublicKey: "b"},
		"not pem":  {PublicKey: "nope"},
		"bad body": {PublicKey: "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----"},
	} {
		if _, err := NewVerifier(cfg); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
