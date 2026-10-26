package gateway

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTConfig struct {
	Secret    string        `env:"JWT_SECRET"`
	PublicKey string        `env:"JWT_PUBLIC_KEY"`
	Issuer    string        `env:"JWT_ISSUER"`
	Audience  string        `env:"JWT_AUDIENCE"`
	Leeway    time.Duration `env:"JWT_LEEWAY" default:"30s"`
}

type Verifier struct {
	key    any
	parser *jwt.Parser
}

func NewVerifier(cfg JWTConfig) (*Verifier, error) {
	var (
		key     any
		methods []string
	)
	switch {
	case cfg.Secret != "" && cfg.PublicKey != "":
		return nil, errors.New("set either JWT_SECRET or JWT_PUBLIC_KEY, not both")
	case cfg.Secret != "":
		key, methods = []byte(cfg.Secret), []string{"HS256", "HS384", "HS512"}
	case cfg.PublicKey != "":
		pub, err := parsePublicKey(cfg.PublicKey)
		if err != nil {
			return nil, err
		}
		key, methods = pub, []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA"}
	default:
		return nil, errors.New("JWT_SECRET or JWT_PUBLIC_KEY is required")
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
	return &Verifier{key: key, parser: jwt.NewParser(opts...)}, nil
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
	if _, err := v.parser.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return v.key, nil }); err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("token has no subject")
	}
	return claims.Subject, nil
}
