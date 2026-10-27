package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Request struct {
	Key         string
	Fingerprint string
}

func New(scope, key string, payload any) (Request, error) {
	if key == "" {
		return Request{}, nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Request{}, err
	}
	return Request{Key: digest(scope, key), Fingerprint: digest(string(body))}, nil
}

func (r Request) Empty() bool {
	return r.Key == ""
}

func (r Request) Verify(storedFingerprint string) error {
	if storedFingerprint != r.Fingerprint {
		return httpx.ErrIdempotencyKeyReused
	}
	return nil
}

func digest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
