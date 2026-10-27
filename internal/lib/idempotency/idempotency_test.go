package idempotency

import (
	"errors"
	"testing"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

type payload struct {
	Item   string `json:"item"`
	Amount int    `json:"amount"`
}

func TestEmptyKeyDisablesIdempotency(t *testing.T) {
	r, err := New("user", "", payload{})
	if err != nil || !r.Empty() {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestKeyIsScoped(t *testing.T) {
	a, _ := New("alice", "k1", payload{Item: "book"})
	b, _ := New("bob", "k1", payload{Item: "book"})
	if a.Key == b.Key {
		t.Fatal("same key for different scopes")
	}
	if a.Fingerprint != b.Fingerprint {
		t.Fatal("fingerprint depends on scope")
	}
}

func TestVerify(t *testing.T) {
	a, _ := New("alice", "k1", payload{Item: "book", Amount: 1})
	same, _ := New("alice", "k1", payload{Item: "book", Amount: 1})
	other, _ := New("alice", "k1", payload{Item: "book", Amount: 2})
	if a.Key != same.Key || a.Verify(same.Fingerprint) != nil {
		t.Fatal("identical requests must match")
	}
	if err := a.Verify(other.Fingerprint); !errors.Is(err, httpx.ErrIdempotencyKeyReused) {
		t.Fatalf("err %v", err)
	}
}
