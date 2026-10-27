package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/golang-jwt/jwt/v5"
)

const minRefreshGap = 10 * time.Second

var errUnknownKey = errors.New("no key with this kid in the JWKS")

type JWKS struct {
	url     string
	refresh time.Duration
	client  *http.Client
	log     *slog.Logger
	now     func() time.Time

	mu        sync.RWMutex
	keys      map[string]any
	fetchedAt time.Time
	fetching  sync.Mutex
}

func NewJWKS(url string, refresh time.Duration, log *slog.Logger) *JWKS {
	return &JWKS{
		url:     url,
		refresh: refresh,
		client:  httpx.NewClient("jwks", 5*time.Second),
		log:     log,
		now:     time.Now,
		keys:    map[string]any{},
	}
}

func (j *JWKS) Run(ctx context.Context) {
	if err := j.Refresh(ctx); err != nil {
		j.log.Error("jwks fetch failed", "url", j.url, "err", err)
	}
	ticker := time.NewTicker(j.refresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := j.Refresh(ctx); err != nil {
				j.log.Error("jwks refresh failed", "url", j.url, "err", err)
			}
		}
	}
}

func (j *JWKS) Refresh(ctx context.Context) error {
	j.fetching.Lock()
	defer j.fetching.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	if err != nil {
		return err
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	keys, err := parseJWKS(body)
	if err != nil {
		return err
	}
	j.mu.Lock()
	j.keys, j.fetchedAt = keys, j.now()
	j.mu.Unlock()
	return nil
}

func (j *JWKS) KeyFunc(t *jwt.Token) (any, error) {
	kid, _ := t.Header["kid"].(string)
	if key, ok := j.lookup(kid); ok {
		return key, nil
	}
	j.mu.RLock()
	recent := j.now().Sub(j.fetchedAt) < minRefreshGap
	j.mu.RUnlock()
	if !recent {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := j.Refresh(ctx); err != nil {
			j.log.Error("jwks refresh for unknown kid failed", "kid", kid, "err", err)
		}
		if key, ok := j.lookup(kid); ok {
			return key, nil
		}
	}
	return nil, errUnknownKey
}

func (j *JWKS) lookup(kid string) (any, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if kid == "" && len(j.keys) == 1 {
		for _, key := range j.keys {
			return key, true
		}
	}
	key, ok := j.keys[kid]
	return key, ok
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func parseJWKS(body []byte) (map[string]any, error) {
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("jwks is not valid JSON: %w", err)
	}
	keys := make(map[string]any, len(set.Keys))
	for _, k := range set.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		key, err := k.publicKey()
		if err != nil {
			return nil, fmt.Errorf("jwks key %q: %w", k.Kid, err)
		}
		keys[k.Kid] = key
	}
	if len(keys) == 0 {
		return nil, errors.New("jwks has no usable keys")
	}
	return keys, nil
}

func (k jwk) publicKey() (any, error) {
	switch k.Kty {
	case "RSA":
		n, err1 := b64Int(k.N)
		e, err2 := b64Int(k.E)
		if err := errors.Join(err1, err2); err != nil {
			return nil, err
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		curve, ok := map[string]elliptic.Curve{"P-256": elliptic.P256(), "P-384": elliptic.P384(), "P-521": elliptic.P521()}[k.Crv]
		if !ok {
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		size := (curve.Params().BitSize + 7) / 8
		x, err1 := b64Bytes(k.X, size)
		y, err2 := b64Bytes(k.Y, size)
		if err := errors.Join(err1, err2); err != nil {
			return nil, err
		}
		point := append(append([]byte{4}, x...), y...)
		pub, err := ecdsa.ParseUncompressedPublicKey(curve, point)
		if err != nil {
			return nil, fmt.Errorf("invalid EC key: %w", err)
		}
		return pub, nil
	case "OKP":
		if k.Crv != "Ed25519" {
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 key")
		}
		return ed25519.PublicKey(raw), nil
	}
	return nil, fmt.Errorf("unsupported key type %q", k.Kty)
}

func b64Bytes(s string, size int) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) == 0 || len(raw) > size {
		return nil, errors.New("invalid base64 coordinate")
	}
	return append(make([]byte, size-len(raw)), raw...), nil
}

func b64Int(s string) (*big.Int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) == 0 {
		return nil, errors.New("invalid base64 number")
	}
	return new(big.Int).SetBytes(raw), nil
}
