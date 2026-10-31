package pg

import (
	"context"
	"fmt"

	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"gorm.io/gorm"
)

func pick(db *gorm.DB) (*gorm.DB, error) { return db, nil }

func TestReadWithoutReplicaUsesPrimary(t *testing.T) {
	w := &gorm.DB{}
	if got, err := Read(context.Background(), &DB{Write: w, Read: w}, pick); err != nil || got != w {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestReadPrefersReplica(t *testing.T) {
	w, r := &gorm.DB{}, &gorm.DB{}
	if got, _ := Read(context.Background(), &DB{Write: w, Read: r}, pick); got != r {
		t.Fatal("expected replica")
	}
}

func TestReadFallsBackToPrimary(t *testing.T) {
	w, r := &gorm.DB{}, &gorm.DB{}
	calls := 0
	got, err := Read(context.Background(), &DB{Write: w, Read: r}, func(db *gorm.DB) (*gorm.DB, error) {
		calls++
		if db == r {
			return nil, errors.New("replica down")
		}
		return db, nil
	})
	if err != nil || got != w || calls != 2 {
		t.Fatalf("got %v err %v calls %d", got, err, calls)
	}
}

func TestReadReturnsPrimaryError(t *testing.T) {
	want := errors.New("boom")
	_, err := Read(context.Background(), &DB{Write: &gorm.DB{}, Read: &gorm.DB{}}, func(*gorm.DB) (int, error) { return 0, want })
	if !errors.Is(err, want) {
		t.Fatalf("err %v", err)
	}
}

func TestFallbackToPrimary(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"not found", gorm.ErrRecordNotFound, true},
		{"network", errors.New("connection refused"), true},
		{"canceled", context.Canceled, false},
		{"deadline", fmt.Errorf("query: %w", context.DeadlineExceeded), false},
		{"recovery conflict", &pgconn.PgError{Code: "40001"}, true},
		{"admin shutdown", &pgconn.PgError{Code: "57P01"}, true},
		{"syntax", &pgconn.PgError{Code: "42601"}, false},
		{"constraint", &pgconn.PgError{Code: "23505"}, false},
	}
	for _, c := range cases {
		if got := fallbackToPrimary(c.err); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestReadInsideTransactionUsesIt(t *testing.T) {
	w, r, tx := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
	ctx := context.WithValue(context.Background(), txKey{}, tx)
	if got, _ := Read(ctx, &DB{Write: w, Read: r}, pick); got != tx {
		t.Fatal("read inside a transaction must use the transaction")
	}
	if got := (&DB{Write: w}).Conn(ctx); got != tx {
		t.Fatal("Conn must return the transaction")
	}
}
