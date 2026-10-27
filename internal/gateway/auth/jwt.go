package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTConfig struct {
	Secret      string        `env:"JWT_SECRET"`
	PublicKey   string        `env:"JWT_PUBLIC_KEY"`
	JWKSURL     string        `env:"JWT_JWKS_URL"`
	JWKSRefresh time.Duration `env:"JWT_JWKS_REFRESH" default:"10m"`
	Issuer      string        `env:"JWT_ISSUER"`
	Audience    string        `env:"JWT_AUDIENCE"`
	Leeway      time.Duration `env:"JWT_LEEWAY" default:"30s"`
}

type Verifier struct {
	keyFunc jwt.Keyfunc
	parser  *jwt.Parser
	jwks    *JWKS
}

func NewVerifier(cfg JWTConfig, log *slog.Logger) (*Verifier, error) {
	sources := 0
	for _, set := range []bool{cfg.Secret != "", cfg.PublicKey != "", cfg.JWKSURL != ""} {
		if set {
			sources++
		}
	}
	switch sources {
	case 0:
		return nil, errors.New("one of JWT_SECRET, JWT_PUBLIC_KEY or JWT_JWKS_URL is required")
	case 1:
	default:
		return nil, errors.New("set only one of JWT_SECRET, JWT_PUBLIC_KEY and JWT_JWKS_URL")
	}

	v := &Verifier{}
	var methods []string
	switch {
	case cfg.Secret != "":
		secret := []byte(cfg.Secret)
		v.keyFunc = func(*jwt.Token) (any, error) { return secret, nil }
		methods = []string{"HS256", "HS384", "HS512"}
	case cfg.PublicKey != "":
		pub, err := parsePublicKey(cfg.PublicKey)
		if err != nil {
			return nil, err
		}
		v.keyFunc = func(*jwt.Token) (any, error) { return pub, nil }
		methods = asymmetricMethods
	default:
		v.jwks = NewJWKS(cfg.JWKSURL, cfg.JWKSRefresh, log)
		v.keyFunc = v.jwks.KeyFunc
		methods = asymmetricMethods
	}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods(methods),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(cfg.Leeway),
	}
	if cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(cfg.Issuer))
	}
	if cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.Audience))
	}
	v.parser = jwt.NewParser(opts...)
	return v, nil
}

var asymmetricMethods = []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA"}

func (v *Verifier) Start(ctx context.Context) {
	if v.jwks != nil {
		go v.jwks.Run(ctx)
	}
}

func parsePublicKey(raw string) (any, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("JWT_PUBLIC_KEY is not PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("JWT_PUBLIC_KEY: %w", err)
	}
	return pub, nil
}

func (v *Verifier) Subject(token string) (string, error) {
	claims := jwt.RegisteredClaims{}
	if _, err := v.parser.ParseWithClaims(token, &claims, v.keyFunc); err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("token has no subject")
	}
	return claims.Subject, nil
}
