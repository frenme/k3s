package pg

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestQueryWithoutReplicaUsesPrimary(t *testing.T) {
	w := &pgxpool.Pool{}
	p := &Pools{Write: w}
	got, err := Query(p, func(pool *pgxpool.Pool) (*pgxpool.Pool, error) { return pool, nil })
	if err != nil || got != w {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestQueryPrefersReplica(t *testing.T) {
	w, r := &pgxpool.Pool{}, &pgxpool.Pool{}
	p := &Pools{Write: w, Read: r}
	got, _ := Query(p, func(pool *pgxpool.Pool) (*pgxpool.Pool, error) { return pool, nil })
	if got != r {
		t.Fatal("expected replica")
	}
}

func TestQueryFallsBackToPrimary(t *testing.T) {
	w, r := &pgxpool.Pool{}, &pgxpool.Pool{}
	p := &Pools{Write: w, Read: r}
	calls := 0
	got, err := Query(p, func(pool *pgxpool.Pool) (*pgxpool.Pool, error) {
		calls++
		if pool == r {
			return nil, errors.New("replica down")
		}
		return pool, nil
	})
	if err != nil || got != w || calls != 2 {
		t.Fatalf("got %v err %v calls %d", got, err, calls)
	}
}

func TestQueryReturnsPrimaryError(t *testing.T) {
	p := &Pools{Write: &pgxpool.Pool{}, Read: &pgxpool.Pool{}}
	want := errors.New("boom")
	_, err := Query(p, func(pool *pgxpool.Pool) (int, error) { return 0, want })
	if !errors.Is(err, want) {
		t.Fatalf("err %v", err)
	}
}
